package webplayer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func fixture(t *testing.T, raw string, opt player.Options) (*Player, *session, <-chan error) {
	t.Helper()
	p, err := New(func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	done, err := p.Play("", playlist.Channel{URL: raw}, opt)
	if err != nil {
		t.Fatal(err)
	}
	return p, p.current, done
}
func request(t *testing.T, p *Player, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, p.origin+target, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	return w
}
func tickPath(p *Player, s *session) string { return p.prefix + "s/" + s.id + "/tick/state" }
func TestSessionReportsControlsAndStop(t *testing.T) {
	var ready, progress atomic.Int32
	p, s, done := fixture(t, "https://example.org/live.m3u8", player.Options{Controls: player.Controls{Volume: 70}, OnReady: func() { ready.Add(1) }, OnPlayback: func(player.PlaybackStatus) { progress.Add(1) }})
	p.SetControls(player.Controls{Volume: 42, Muted: true, Paused: true})
	for i := 0; i < 2; i++ {
		w := request(t, p, "POST", tickPath(p, s), `{"ready":true,"position":1,"paused":true,"buffering":false}`, nil)
		var c command
		if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		source, err := url.Parse(c.Source)
		if err != nil || !source.IsAbs() {
			t.Fatal("worker media URL must be absolute")
		}
		if c.Volume != 42 || !c.Muted || !c.Paused || c.Kind != "hls" {
			t.Fatalf("bad controls: %+v", c)
		}
	}
	if ready.Load() != 1 || progress.Load() != 2 {
		t.Fatal("readiness not one-shot or progress missing")
	}
	p.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish session")
	}
	if request(t, p, "POST", tickPath(p, s), `{}`, nil).Code != 404 {
		t.Fatal("stale session remained accessible")
	}
}
func TestSecurityAndProtocolBoundary(t *testing.T) {
	p, s, _ := fixture(t, "https://example.org/live", player.Options{})
	for _, tc := range []struct {
		path, origin string
		want         int
	}{
		{"/player.html", "", 404}, {p.prefix + "player.html", "https://evil.test", 403},
		{p.prefix + "s/" + s.id + "/media/999", "", 404},
	} {
		if got := request(t, p, "GET", tc.path, "", map[string]string{"Origin": tc.origin}).Code; got != tc.want {
			t.Fatalf("%s: %d", tc.path, got)
		}
	}
	r := httptest.NewRequest("GET", p.origin+p.prefix+"player.html", nil)
	r.Host = "evil.test"
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("DNS rebinding host accepted")
	}
	for _, raw := range []string{"file:///etc/passwd", "udp://239.1.1.1:1234", "http://u:p@example.org/live"} {
		if _, err := p.Play("", playlist.Channel{URL: raw}); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, body := range []string{`{"position":-1}`, `{"position":1,"url":"http://evil"}`, strings.Repeat("x", 4096)} {
		if request(t, p, "POST", tickPath(p, s), body, nil).Code != 400 {
			t.Fatal("invalid report accepted")
		}
	}
	w = request(t, p, "GET", p.prefix+"player.html", "", nil)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("page or CSP missing")
	}
	for _, asset := range []string{"player.js", "vendor/hls.min.js", "vendor/mpegts.js", "vendor/hls-LICENSE.txt", "vendor/mpegts-LICENSE.txt"} {
		if request(t, p, "GET", p.prefix+asset, "", nil).Code != 200 {
			t.Fatalf("missing bundled %s", asset)
		}
	}
}
func TestNewSessionInvalidatesOldSession(t *testing.T) {
	p, old, done := fixture(t, "https://example.org/one", player.Options{})
	if _, err := p.Play("", playlist.Channel{URL: "https://example.org/two.flv"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	default:
		t.Fatal("old session not stopped")
	}
	if request(t, p, "POST", tickPath(p, old), `{"ready":true}`, nil).Code != 404 {
		t.Fatal("stale report accepted")
	}
	if p.current.kind != "flv" {
		t.Fatal("FLV not selected")
	}
}
func TestFatalReportFinishesSession(t *testing.T) {
	p, s, done := fixture(t, "https://example.org/live", player.Options{})
	request(t, p, "POST", tickPath(p, s), `{"failure":"hls-fatal"}`, nil)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("fatal error hidden")
		}
	case <-time.After(time.Second):
		t.Fatal("fatal report did not finish")
	}
}
func TestRelayHeadersRedirectRangeAndHLS(t *testing.T) {
	var base string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "test-agent" || r.Header.Get("Referer") != "https://source.test/" {
			t.Error("route headers missing")
		}
		switch r.URL.Path {
		case "/entry":
			http.Redirect(w, r, base+"/lists/live.m3u8", 302)
		case "/lists/live.m3u8":
			io.WriteString(w, "\ufeff#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"../key?k=secret\"\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\n../segment.ts?token=secret\n")
		case "/segment.ts":
			if r.Header.Get("Range") != "bytes=0-3" {
				t.Error("range missing")
			}
			w.Header().Set("Content-Range", "bytes 0-3/20")
			w.Header().Set("Content-Type", "video/mp2t")
			w.Header().Set("Set-Cookie", "secret=1")
			w.WriteHeader(206)
			io.WriteString(w, "data")
		default:
			io.WriteString(w, "key")
		}
	}))
	defer upstream.Close()
	base = upstream.URL
	p, s, _ := fixture(t, base+"/entry", player.Options{})
	s.headers = playlist.StreamHeaders{UserAgent: "test-agent", Referer: "https://source.test/"}
	w := request(t, p, "GET", p.prefix+"s/"+s.id+"/media/0", "", nil)
	body := w.Body.String()
	if w.Code != 200 || strings.Contains(body, base) || strings.Contains(body, "secret") || !strings.Contains(body, `URI="`+p.prefix) {
		t.Fatalf("unsafe manifest: %s", body)
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	segment := lines[len(lines)-1]
	w = request(t, p, "GET", segment, "", map[string]string{"Range": "bytes=0-3"})
	if w.Code != 206 || w.Body.String() != "data" || w.Header().Get("Content-Range") == "" || w.Header().Get("Set-Cookie") != "" {
		t.Fatal("range or header boundary failed")
	}
}
func TestManifestReferencesAndBounds(t *testing.T) {
	p, s, _ := fixture(t, "https://example.org/live", player.Options{})
	base, _ := url.Parse("https://example.org/a/index.m3u8")
	for _, raw := range []string{"#EXTM3U\nfile:///etc/passwd", "#EXTM3U\n#EXT-X-KEY:URI=\"data:secret\"", "#EXTM3U\n{$name}.ts"} {
		if _, err := p.rewriteManifest(s, base, raw); err == nil {
			t.Fatalf("accepted unsafe manifest %q", raw)
		}
	}
	for i := 0; i < 8200; i++ {
		// Old references are reclaimable, so live viewing does not hit a fixed
		// lifetime segment budget; actively retained references remain bounded.
		s.refs["old"+strconv.Itoa(i)] = mediaRef{url: "old", used: time.Now().Add(-time.Hour)}
	}
	if _, err := p.mediaURL(s, base, "new.ts"); err != nil {
		t.Fatal(err)
	}
	if len(s.refs) > 8192 {
		t.Fatal("reference map not reclaimed")
	}
}
func TestRedirectDoesNotInventSensitiveReferer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/entry" {
			http.Redirect(w, r, "/media", 302)
			return
		}
		if got := r.Header.Get("Referer"); got != "" {
			t.Errorf("redirect leaked media URL: %q", got)
		}
		io.WriteString(w, "media")
	}))
	defer upstream.Close()
	p, s, _ := fixture(t, upstream.URL+"/entry?token=secret", player.Options{})
	if w := request(t, p, "GET", p.prefix+"s/"+s.id+"/media/0", "", nil); w.Code != 200 {
		t.Fatalf("relay failed: %d", w.Code)
	}
}

func TestStopCancelsUpstream(t *testing.T) {
	cancelled := make(chan struct{})
	entered := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(cancelled) }))
	defer upstream.Close()
	p, s, _ := fixture(t, upstream.URL, player.Options{})
	finished := make(chan struct{})
	go func() { defer close(finished); request(t, p, "GET", p.prefix+"s/"+s.id+"/media/0", "", nil) }()
	<-entered
	p.Stop()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("upstream request survived Stop")
	}
	<-finished
}
