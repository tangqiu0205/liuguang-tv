package app

import (
	"bytes"
	"container/list"
	"image"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

const (
	logoCacheLimit = 512
	logoRetryDelay = 30 * time.Second
	logoSize       = 96 // Enough for the 30 DIP icon on high-DPI screens.
)

type logoEntry struct {
	url     string
	bitmap  *ui.Bitmap
	loading bool
	retryAt time.Time
	timer   *time.Timer
}

// logoCache bounds concurrent requests and uses LRU eviction for retained images.
type logoCache struct {
	mu     sync.Mutex
	items  map[string]*list.Element
	lru    list.List
	active int
}

func (cache *logoCache) get(raw string, services ui.Services) *ui.Bitmap {
	if raw == "" {
		return nil
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if element := cache.items[raw]; element != nil {
		cache.lru.MoveToFront(element)
		entry := element.Value.(*logoEntry)
		if entry.bitmap != nil {
			return entry.bitmap
		}
		if entry.loading || time.Now().Before(entry.retryAt) || cache.active >= 4 {
			return nil
		}
		cache.start(entry, services)
		return nil
	}
	if cache.active >= 4 {
		return nil
	}
	if len(cache.items) >= logoCacheLimit {
		for element := cache.lru.Back(); element != nil; element = element.Prev() {
			entry := element.Value.(*logoEntry)
			if !entry.loading {
				if entry.timer != nil {
					entry.timer.Stop()
				}
				delete(cache.items, entry.url)
				cache.lru.Remove(element)
				break
			}
		}
	}
	if cache.items == nil {
		cache.items = make(map[string]*list.Element)
	}
	entry := &logoEntry{url: raw}
	cache.items[raw] = cache.lru.PushFront(entry)
	cache.start(entry, services)
	return nil
}

// start is called with cache.mu held; loading entries are never evicted.
func (cache *logoCache) start(entry *logoEntry, services ui.Services) {
	entry.loading = true
	cache.active++
	go func() {
		bitmap := fetchLogo(entry.url)
		cache.mu.Lock()
		entry.bitmap = bitmap
		entry.loading = false
		cache.active--
		if bitmap == nil {
			entry.retryAt = time.Now().Add(logoRetryDelay)
			entry.timer = time.AfterFunc(logoRetryDelay, services.Invalidate)
		}
		cache.mu.Unlock()
		services.Invalidate()
	}()
}

func fetchLogo(raw string) *ui.Bitmap {
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !validLogoURL(req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	u, err := url.Parse(raw)
	if err != nil || !validLogoURL(u) {
		return nil
	}
	resp, err := client.Get(raw)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 2048 || config.Height > 2048 || int64(config.Width)*int64(config.Height) > 4<<20 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	width, height := config.Width, config.Height
	if width > logoSize || height > logoSize {
		if width >= height {
			height = max(1, height*logoSize/width)
			width = logoSize
		} else {
			width = max(1, width*logoSize/height)
			height = logoSize
		}
	}
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)
	return ui.NewBitmap(scaled)
}

func validLogoURL(u *url.URL) bool {
	return u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}
