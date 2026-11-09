// Package epg reads bounded XMLTV guides independently of stream playback.
package epg

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const MaxBytes = 64 << 20
const maxProgrammes = 300000

var (
	ErrXML          = errors.New("XMLTV 格式无效")
	ErrTooLarge     = errors.New("EPG 超过大小限制（64 MiB）")
	ErrNoProgrammes = errors.New("没有有效节目（起止时间或时长不合法）")
	ErrNetwork      = errors.New("下载失败，请检查网络和地址")
	ErrGzip         = errors.New("gzip 文件损坏或不完整")
	ErrCache        = errors.New("缓存写入失败，请检查磁盘空间和权限")
	ErrLocalFile    = errors.New("无法读取本地 EPG 文件")
)

type HTTPError int

func (e HTTPError) Error() string { return fmt.Sprintf("EPG HTTP %d", int(e)) }

// FailureReason never exposes a URL, authentication token or local path.
func FailureReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "EPG 更新超时"
	}
	if errors.Is(err, context.Canceled) {
		return "EPG 更新已取消"
	}
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Error()
	}
	for _, reason := range []error{ErrExpired, ErrTooLarge, ErrNoProgrammes, ErrXML, ErrNetwork, ErrGzip, ErrCache, ErrLocalFile} {
		if errors.Is(err, reason) {
			return reason.Error()
		}
	}
	return "EPG 读取失败"
}

type Programme struct {
	Start       time.Time `xml:"-"`
	Stop        time.Time `xml:"-"`
	Title       string    `xml:"title"`
	Description string    `xml:"desc"`
}
type Guide struct {
	Programmes map[string][]Programme
	Names      map[string]string
}

func nameKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-_.·", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// XMLTV timestamps without an explicit timezone use UTC.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"20060102150405 -0700", "200601021504 -0700", "20060102150405", "200601021504"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无效的 XMLTV 时间")
}

func Parse(r io.Reader) (guide *Guide, err error) {
	limited := &io.LimitedReader{R: r, N: MaxBytes + 1}
	defer func() {
		if limited.N <= 0 {
			guide = nil
			err = ErrTooLarge
		}
	}()
	d := xml.NewDecoder(limited)
	g := &Guide{Programmes: map[string][]Programme{}, Names: map[string]string{}}
	root, closed, count := false, false, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrXML, err)
		}
		if end, ok := token.(xml.EndElement); ok && end.Name.Local == "tv" {
			closed = true
			continue
		}
		if text, ok := token.(xml.CharData); ok && strings.TrimSpace(string(text)) != "" {
			return nil, ErrXML
		}
		el, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if closed {
			return nil, ErrXML
		}
		if !root {
			if el.Name.Local != "tv" {
				return nil, ErrXML
			}
			root = true
			continue
		}
		switch el.Name.Local {
		case "channel":
			var ch struct {
				ID    string   `xml:"id,attr"`
				Names []string `xml:"display-name"`
			}
			if err := d.DecodeElement(&ch, &el); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrXML, err)
			}
			id := strings.ToLower(strings.TrimSpace(ch.ID))
			for _, name := range ch.Names {
				key := nameKey(name)
				if key == "" || id == "" {
					continue
				}
				if old, exists := g.Names[key]; exists && old != id {
					g.Names[key] = ""
				} else if !exists {
					g.Names[key] = id
				}
			}
		case "programme":
			count++
			if count > maxProgrammes {
				return nil, fmt.Errorf("%w: 节目数量超过限制", ErrTooLarge)
			}
			var p struct {
				Channel     string `xml:"channel,attr"`
				Start       string `xml:"start,attr"`
				Stop        string `xml:"stop,attr"`
				Title       string `xml:"title"`
				Description string `xml:"desc"`
			}
			if err := d.DecodeElement(&p, &el); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrXML, err)
			}
			start, e1 := parseTime(p.Start)
			stop, e2 := parseTime(p.Stop)
			id := strings.ToLower(strings.TrimSpace(p.Channel))
			if e1 != nil || e2 != nil || id == "" || !stop.After(start) || stop.Sub(start) > MaxProgrammeDuration {
				continue
			}
			g.Programmes[id] = append(g.Programmes[id], Programme{start, stop, strings.TrimSpace(p.Title), strings.TrimSpace(p.Description)})
		default:
			if err := d.Skip(); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrXML, err)
			}
		}
	}
	if limited.N <= 0 {
		return nil, ErrTooLarge
	}
	if !root || !closed {
		return nil, ErrXML
	}
	if len(g.Programmes) == 0 {
		return nil, ErrNoProgrammes
	}
	for id, ps := range g.Programmes {
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].Start.Before(ps[j].Start) })
		g.Programmes[id] = ps
	}
	return g, nil
}

func (g *Guide) Schedule(id, name string) []Programme {
	if g == nil {
		return nil
	}
	if ps := g.Programmes[strings.ToLower(strings.TrimSpace(id))]; len(ps) > 0 {
		return ps
	}
	return g.Programmes[g.Names[nameKey(name)]]
}

func NowNext(ps []Programme, now time.Time) (current, next *Programme) {
	// Schedules are sorted at parse time. Live lookups normally need one
	// binary search and one stop-time check, even for very large guides.
	split := sort.Search(len(ps), func(i int) bool { return ps[i].Start.After(now) })
	if split < len(ps) {
		next = &ps[split]
	}
	// Walk backwards to preserve the latest active programme when an upstream
	// source supplies overlapping intervals, including an already-ended overlap.
	for i := split - 1; i >= 0; i-- {
		if now.Before(ps[i].Stop) {
			current = &ps[i]
			break
		}
	}
	return
}

func cachePath(dir, source string) string {
	return filepath.Join(dir, fmt.Sprintf("%x.xml", sha256.Sum256([]byte(source))))
}
func Cached(dir, source string) (*Guide, error) {
	f, err := os.Open(cachePath(dir, source))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Load validates a downloaded guide before replacing the previous cache.
func Load(ctx context.Context, source, dir string) (*Guide, error) {
	return load(ctx, source, dir, nil)
}

// LoadRetained rejects wholly expired downloads before replacing a useful cache.
func LoadRetained(ctx context.Context, source, dir string, now time.Time) (*Guide, error) {
	return load(ctx, source, dir, &now)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

type cacheWriter struct {
	w   io.Writer
	err error
}

func (w *cacheWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if err != nil {
		w.err = err
	}
	return n, err
}

func load(ctx context.Context, source, dir string, now *time.Time) (*Guide, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var body io.ReadCloser
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, ErrNetwork
		}
		resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				return nil, context.DeadlineExceeded
			}
			return nil, ErrNetwork
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, HTTPError(resp.StatusCode)
		}
		body = resp.Body
	} else {
		f, err := os.Open(source)
		if err != nil {
			return nil, ErrLocalFile
		}
		body = f
	}
	defer body.Close()
	compressed := &io.LimitedReader{R: contextReader{ctx, body}, N: MaxBytes + 1}
	reader := bufio.NewReader(compressed)
	var input io.Reader = reader
	magic, _ := reader.Peek(2)
	compressedGzip := len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b
	if compressedGzip {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, ErrGzip
		}
		defer gz.Close()
		input = gz
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, ErrCache
	}
	f, err := os.CreateTemp(dir, "epg-*.tmp")
	if err != nil {
		return nil, ErrCache
	}
	defer os.Remove(f.Name())
	writer := &cacheWriter{w: f}
	g, parseErr := Parse(io.TeeReader(input, writer))
	closeErr := f.Close()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if compressed.N <= 0 {
		return nil, ErrTooLarge
	}
	if writer.err != nil {
		return nil, ErrCache
	}
	if parseErr != nil {
		var timeout net.Error
		if errors.As(parseErr, &timeout) && timeout.Timeout() {
			return nil, context.DeadlineExceeded
		}
		if errors.Is(parseErr, gzip.ErrChecksum) || errors.Is(parseErr, gzip.ErrHeader) || (compressedGzip && errors.Is(parseErr, io.ErrUnexpectedEOF)) {
			return nil, ErrGzip
		}
		return nil, parseErr
	}
	if now != nil {
		g = g.Retained(*now)
		if len(g.Programmes) == 0 {
			return nil, ErrExpired
		}
	}
	if closeErr != nil {
		return g, ErrCache
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Rename(f.Name(), cachePath(dir, source)); err != nil {
		return g, ErrCache
	}
	return g, nil
}
