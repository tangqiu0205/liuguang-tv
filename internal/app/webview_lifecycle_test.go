package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"github.com/tangqiu0205/LiuguangTV/internal/webplayer"
)

func attachWebPlayer(t *testing.T, a *application) (*webplayer.Player, string, <-chan error) {
	t.Helper()
	var pageURL string
	engine, err := webplayer.New(func(raw string) error { pageURL = raw; return nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Close)
	done, err := engine.Play("", playlist.Channel{URL: "http://127.0.0.1:1/demo.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	a.state.PlaybackEngine = "webview"
	a.player, a.webPlayer = engine, engine
	return engine, pageURL, done
}

func TestLeavingWebViewClosesServiceAndClearsReferences(t *testing.T) {
	for _, action := range []string{"select-mpv", "import-default-mpv", "import-explicit-mpv", "cleanup"} {
		t.Run(action, func(t *testing.T) {
			a, _, _, _ := playbackFixture(t)
			old, pageURL, done := attachWebPlayer(t, a)
			client := &http.Client{Timeout: time.Second}
			defer client.CloseIdleConnections()
			response, err := client.Get(pageURL)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("service was not available: %d", response.StatusCode)
			}
			switch action {
			case "select-mpv":
				a.setPlaybackEngine("mpv")
			case "import-default-mpv":
				a.installImportedConfig(store.State{}, false)
			case "import-explicit-mpv":
				a.installImportedConfig(store.State{PlaybackEngine: "mpv"}, false)
			case "cleanup":
				a.cleanup()
			}
			if a.webPlayer != nil || a.webView != nil || a.player != nil {
				t.Fatal("WebView engine references retained")
			}
			select {
			case <-done:
			default:
				t.Fatal("old session was not stopped")
			}
			if response, err := client.Get(pageURL); err == nil {
				response.Body.Close()
				t.Fatal("local service still accepting requests")
			}
			if _, err := old.Play("", playlist.Channel{URL: "http://127.0.0.1:1/demo.mp4"}); err == nil {
				t.Fatal("closed engine accepted playback")
			}
			a.releaseWebPlayback() // Repeated cleanup must remain safe.
			if action != "cleanup" {
				a.setPlaybackEngine("webview")
				if a.webPlayer != nil || a.webView != nil || a.player != nil {
					t.Fatal("engine was eagerly created on selection")
				}
			}
		})
	}
}

func TestStoppingOrImportingWebViewRetainsReusableEngine(t *testing.T) {
	for _, action := range []string{"stop", "import-webview"} {
		t.Run(action, func(t *testing.T) {
			a, _, _, _ := playbackFixture(t)
			engine, _, done := attachWebPlayer(t, a)
			if action == "stop" {
				a.stop()
			} else {
				a.installImportedConfig(store.State{PlaybackEngine: "webview"}, false)
			}
			select {
			case <-done:
			default:
				t.Fatal("old session was not stopped")
			}
			if a.webPlayer != engine {
				t.Fatal("WebView unnecessarily destroyed while still selected")
			}
			if err := a.ensurePlaybackEngine(); err != nil || a.player != engine {
				t.Fatalf("engine was not reusable: %v", err)
			}
			a.releaseWebPlayback()
		})
	}
}
