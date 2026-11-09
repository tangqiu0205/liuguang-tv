// Package webplayer serves a private, playback-only page and session-scoped media
// relay. It has no dependency on the native UI and never exposes a URL proxy API.
package webplayer

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

//go:embed assets
var assets embed.FS

type Player struct {
	mu             sync.Mutex
	current        *session
	server         *http.Server
	client         *http.Client
	origin, prefix string
	load           func(string) error
	closed         bool
}
type session struct {
	mu       sync.Mutex
	id       string
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan error
	once     sync.Once
	controls player.Controls
	options  player.Options
	ready    bool
	kind     string
	headers  playlist.StreamHeaders
	refs     map[string]mediaRef
	byURL    map[string]string
	next     uint64
}
type mediaRef struct {
	url  string
	used time.Time
}
type report struct {
	Ready     bool    `json:"ready"`
	Position  float64 `json:"position"`
	Paused    bool    `json:"paused"`
	Buffering bool    `json:"buffering"`
	Failure   string  `json:"failure"`
}
type command struct {
	Source string  `json:"source"`
	Kind   string  `json:"kind"`
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted"`
	Paused bool    `json:"paused"`
}

func token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// New starts a loopback-only server. load must navigate the owning WebView;
// Play/Stop are called by the UI thread, while reports arrive on HTTP workers.
func New(load func(string) error) (*Player, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("无法启动本机播放服务")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 15 * time.Second
	p := &Player{origin: "http://" + listener.Addr().String(), prefix: "/" + token() + "/", load: load}
	p.client = &http.Client{Transport: transport, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 8 || !httpURL(r.URL) {
			return errors.New("invalid media redirect")
		}
		// net/http otherwise invents a Referer from the previous media URL,
		// potentially leaking signed query parameters to another CDN.
		r.Header.Del("Referer")
		if value := via[0].Header.Get("Referer"); value != "" {
			r.Header.Set("Referer", value)
		}
		return nil
	}}
	p.server = &http.Server{Handler: p, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		if err := p.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.mu.Lock()
			p.closed = true
			s := p.current
			p.mu.Unlock()
			if s != nil {
				s.finish(errors.New("本机播放服务中断"))
			}
		}
	}()
	return p, nil
}
func httpURL(u *url.URL) bool {
	return u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

// SupportsURL describes the same address boundary used by Play, so callers can
// skip incompatible routes without treating them as an engine startup failure.
func SupportsURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && httpURL(u)
}

func (p *Player) Play(_ string, ch playlist.Channel, options ...player.Options) (<-chan error, error) {
	if !SupportsURL(ch.URL) {
		return nil, errors.New("WebView 仅支持 HTTP(S) 播放地址，请改用 mpv")
	}
	if err := ch.Headers.Validate(); err != nil {
		return nil, err
	}
	u, _ := url.Parse(ch.URL) // Validated by SupportsURL above.
	opt := player.Options{Controls: player.Controls{Volume: 70}}
	if len(options) > 0 {
		opt = options[0]
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{id: token(), ctx: ctx, cancel: cancel, done: make(chan error, 1), controls: opt.Controls, options: opt, headers: ch.Headers, refs: map[string]mediaRef{}, byURL: map[string]string{}}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".m3u8", ".m3u":
		s.kind = "hls"
	case ".flv":
		s.kind = "flv"
	case ".ts", ".m2ts":
		s.kind = "mpegts"
	case ".mp4", ".webm", ".ogg":
		s.kind = "native"
	default:
		s.kind = "auto"
	}
	s.refs["0"] = mediaRef{url: ch.URL, used: time.Now()}
	s.byURL[ch.URL] = "0"
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		cancel()
		return nil, errors.New("本机播放服务已关闭")
	}
	old := p.current
	p.current = s
	p.mu.Unlock()
	if old != nil {
		old.finish(nil)
	}
	if err := p.load(p.origin + p.prefix + "player.html?session=" + s.id); err != nil {
		s.finish(err)
		p.mu.Lock()
		if p.current == s {
			p.current = nil
		}
		p.mu.Unlock()
		return nil, errors.New("无法加载内置播放页面")
	}
	return s.done, nil
}
func (s *session) finish(err error) { s.once.Do(func() { s.cancel(); s.done <- err; close(s.done) }) }
func (p *Player) SetControls(c player.Controls) {
	p.mu.Lock()
	s := p.current
	p.mu.Unlock()
	if s != nil {
		s.mu.Lock()
		s.controls = c
		s.mu.Unlock()
	}
}
func (p *Player) Stop() {
	p.mu.Lock()
	s := p.current
	p.current = nil
	p.mu.Unlock()
	if s != nil {
		s.finish(nil)
		_ = p.load("about:blank")
	}
}
func (p *Player) Close() {
	p.Stop()
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	_ = p.server.Close()
	p.client.CloseIdleConnections()
}

func (p *Player) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Exact Host blocks DNS rebinding; no CORS, cookies, external scripts or
	// arbitrary target parameters. Tokens are capabilities, never logged.
	if r.Host != strings.TrimPrefix(p.origin, "http://") || !strings.HasPrefix(r.URL.Path, p.prefix) {
		http.NotFound(w, r)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != p.origin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; media-src 'self' blob:; worker-src 'self' blob:; frame-ancestors 'none'; base-uri 'none'")
	name := strings.TrimPrefix(r.URL.Path, p.prefix)
	if strings.HasPrefix(name, "s/") {
		parts := strings.Split(name, "/")
		p.mu.Lock()
		s := p.current
		p.mu.Unlock()
		if len(parts) != 4 || s == nil || parts[1] != s.id || s.ctx.Err() != nil {
			http.NotFound(w, r)
			return
		}
		if parts[2] == "tick" && parts[3] == "state" {
			p.tick(w, r, s)
			return
		}
		if parts[2] == "media" {
			p.relay(w, r, s, parts[3])
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// FileServer is rooted at the embedded assets, never the host filesystem.
	files, _ := fs.Sub(assets, "assets")
	http.StripPrefix(p.prefix, http.FileServer(http.FS(files))).ServeHTTP(w, r)
}
func (p *Player) tick(w http.ResponseWriter, r *http.Request, s *session) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var v report
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v); err != nil || math.IsNaN(v.Position) || math.IsInf(v.Position, 0) || v.Position < 0 {
		http.Error(w, "invalid report", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	c := s.controls
	notify := v.Ready && !s.ready
	s.ready = s.ready || v.Ready
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	// Blob-based mpegts workers have no document base URL: keep this absolute.
	_ = json.NewEncoder(w).Encode(command{Source: p.origin + p.prefix + "s/" + s.id + "/media/0", Kind: s.kind, Volume: max(0, min(100, c.Volume)), Muted: c.Muted, Paused: c.Paused})
	if v.Failure != "" {
		s.finish(errors.New("WebView 播放失败，请检查格式或改用 mpv"))
		return
	}
	if notify && s.options.OnReady != nil {
		s.options.OnReady()
	}
	if s.options.OnPlayback != nil {
		s.options.OnPlayback(player.PlaybackStatus{Position: &v.Position, Paused: &v.Paused, Buffering: &v.Buffering})
	}
}
