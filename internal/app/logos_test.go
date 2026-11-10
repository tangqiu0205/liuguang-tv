package app

import (
	"container/list"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestLogoCacheRetriesFailedRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "temporary error", http.StatusServiceUnavailable)
			return
		}
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	}))
	defer server.Close()
	cache := &logoCache{}
	cache.get(server.URL, ui.Services{})
	waitLogoIdle(t, cache)
	cache.get(server.URL, ui.Services{})
	if requests.Load() != 1 {
		t.Fatal("failed request was retried before the delay")
	}
	cache.mu.Lock()
	entry := cache.items[server.URL].Value.(*logoEntry)
	entry.timer.Stop()
	entry.retryAt = time.Time{}
	cache.mu.Unlock()
	cache.get(server.URL, ui.Services{})
	waitLogoIdle(t, cache)
	if requests.Load() != 2 || cache.get(server.URL, ui.Services{}) == nil {
		t.Fatal("failed logo did not recover on retry")
	}
}

func TestLogoCacheEvictsLeastRecentlyUsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}))
	defer server.Close()
	cache := &logoCache{items: make(map[string]*list.Element)}
	bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	for i := range logoCacheLimit {
		key := strconv.Itoa(i)
		cache.items[key] = cache.lru.PushFront(&logoEntry{url: key, bitmap: bitmap})
	}
	cache.get("0", ui.Services{}) // Keep the oldest entry.
	cache.get(server.URL, ui.Services{})
	waitLogoIdle(t, cache)
	if len(cache.items) != logoCacheLimit || cache.items["0"] == nil || cache.items["1"] != nil || cache.get(server.URL, ui.Services{}) == nil {
		t.Fatal("cache did not evict the least recently used entry for a new logo")
	}
}

func TestFetchLogoScalesLargeImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 400))
	img.Set(10, 10, color.White)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = png.Encode(w, img)
	}))
	defer server.Close()
	bitmap := fetchLogo(server.URL)
	if bitmap == nil {
		t.Fatal("valid image failed to load")
	}
	if w, h := bitmap.Size(); w != logoSize || h != logoSize/2 {
		t.Fatalf("bitmap size = %dx%d, want %dx%d", w, h, logoSize, logoSize/2)
	}
}

func waitLogoIdle(t *testing.T, cache *logoCache) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		active := cache.active
		cache.mu.Unlock()
		if active == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("logo request did not finish")
}
