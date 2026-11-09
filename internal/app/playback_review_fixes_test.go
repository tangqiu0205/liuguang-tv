package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/webplayer"
)

func TestIdleVolumeControlsDoNotCreatePlayer(t *testing.T) {
	for _, mode := range []string{"", "mpv", "webview"} {
		t.Run(mode, func(t *testing.T) {
			a, p, _, ch := playbackFixture(t)
			a.state.PlaybackEngine = mode
			a.player = nil
			if !a.supportsPlaybackControls() {
				t.Skip("mpv controls not available on this platform")
			}
			tt := ui.NewTester(a.view, 1280, 820)
			if err := tt.Click("静音"); err != nil {
				t.Fatal(err)
			}
			if !a.controls.Muted {
				t.Fatal("idle mute was not retained")
			}
			if err := tt.Click("音量"); err != nil {
				t.Fatal(err)
			}
			if a.controls.Volume == 70 {
				t.Fatal("idle slider did not change volume")
			}
			a.changeVolume(-200)
			if a.controls.Volume != 0 {
				t.Fatal("idle volume not clamped")
			}
			a.changeVolume(200)
			if a.controls.Volume != 100 {
				t.Fatal("idle volume not clamped")
			}
			// An engine switch discards the player again; controls must remain safe.
			next := "mpv"
			if mode != "webview" {
				next = "webview"
			}
			a.setPlaybackEngine(next)
			if !a.supportsPlaybackControls() {
				a.setPlaybackEngine("webview")
			}
			tt.Frame()
			if err := tt.Click("取消静音"); err != nil {
				t.Fatal(err)
			}
			if a.controls.Muted {
				t.Fatal("idle unmute failed after switch")
			}
			a.epg.show = true
			tt.SetSize(1000, 620)
			if err := tt.Click("更多播放操作"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem("音量 25%"); err != nil {
				t.Fatal(err)
			}
			if a.controls.Volume != 25 {
				t.Fatal("idle menu did not retain volume")
			}
			if a.player != nil || a.webPlayer != nil || a.webView != nil {
				t.Fatal("idle controls eagerly created an engine")
			}
			a.player = p
			a.play(ch)
			if len(p.streams) != 1 || p.streams[0].opt.Controls.Volume != 25 || p.streams[0].opt.Controls.Muted {
				t.Fatal("next playback did not inherit idle controls")
			}
		})
	}
}

func mixedProtocolChannel() playlist.Channel {
	return playlist.Merge([]playlist.Channel{
		{Name: "Mixed", URL: "http://127.0.0.1:1/first.m3u8"},
		{Name: "Mixed", URL: "rtsp://127.0.0.1:1/live"},
		{Name: "Mixed", URL: "udp://239.1.1.1:1234"},
		{Name: "Mixed", URL: "https://example.org/last.m3u8"},
	})[0]
}

func TestWebViewSkipsUnsupportedPreferredAndManualRoutes(t *testing.T) {
	for _, manual := range []bool{false, true} {
		name := "preferred"
		if manual {
			name = "manual"
		}
		t.Run(name, func(t *testing.T) {
			a, p, _, _ := playbackFixture(t)
			a.state.PlaybackEngine = "webview"
			ch := mixedProtocolChannel()
			a.state.PreferredLines = map[string]string{playlist.ChannelKey(ch): ch.Lines[1].URL}
			if manual {
				a.startLine(ch, 1, map[string]bool{})
			} else {
				a.play(ch)
			}
			if len(p.streams) != 1 || p.streams[0].url != ch.Lines[0].URL {
				t.Fatal("unsupported selection blocked a compatible route")
			}
			if !a.triedLines[ch.Lines[1].URL] {
				t.Fatal("unsupported route was not recorded")
			}
			if a.lineIndex != 0 {
				t.Fatal("original route index changed")
			}
		})
	}
}

func TestWebViewMixedProtocolFailoverUsesRemainingHTTPAndStops(t *testing.T) {
	a, _, queue, _ := playbackFixture(t)
	a.state.PlaybackEngine = "webview"
	var loads int
	engine, err := webplayer.New(func(raw string) error {
		if raw != "about:blank" {
			loads++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	a.player, a.webPlayer = engine, engine
	ch := mixedProtocolChannel()
	a.play(ch)
	if a.nowURL != ch.Lines[0].URL {
		t.Fatalf("first route did not start: %s", a.status)
	}
	first := a.generation
	a.lineTimedOut(first)
	if loads != 2 || a.nowURL != ch.Lines[3].URL || a.lineIndex != 3 {
		t.Fatalf("mixed protocol failover stopped early: loads=%d url=%s status=%s", loads, a.nowURL, a.status)
	}
	for _, line := range ch.Lines {
		if !a.triedLines[line.URL] {
			t.Fatalf("route not attempted or skipped: %s", line.URL)
		}
	}
	// Ignore the previous session's delayed completion after starting its replacement.
	select {
	case fn := <-queue:
		fn()
	default:
	}
	if a.nowURL != ch.Lines[3].URL {
		t.Fatal("stale completion changed current route")
	}
	a.lineTimedOut(a.generation)
	if a.now != "" || loads != 2 || a.startupTimer != nil || !strings.Contains(a.status, "所有线路") {
		t.Fatalf("exhausted attempts did not stop: %s", a.status)
	}
}

func TestWebViewOnlyUnsupportedRoutesDoesNotCreateEngine(t *testing.T) {
	a, _, _, _ := playbackFixture(t)
	a.state.PlaybackEngine = "webview"
	a.player = nil
	ch := playlist.Merge([]playlist.Channel{{Name: "Unsupported", URL: "rtsp://127.0.0.1/live"}, {Name: "Unsupported", URL: "udp://239.1.1.1:1234"}})[0]
	tried := map[string]bool{}
	if a.startLine(ch, 0, tried) {
		t.Fatal("unsupported channel started")
	}
	if len(tried) != 2 || a.player != nil || a.webPlayer != nil || a.startupTimer != nil || !strings.Contains(a.status, "兼容 HTTP(S)") {
		t.Fatalf("unsupported routes not bounded or clearly reported: %s", a.status)
	}
}

func TestMPVStillAcceptsNonHTTPRoutesAndStartupFailuresStillStop(t *testing.T) {
	a, p, _, _ := playbackFixture(t)
	ch := mixedProtocolChannel()
	if !a.startLine(ch, 1, map[string]bool{}) || p.streams[0].url != ch.Lines[1].URL {
		t.Fatal("mpv lost RTSP support")
	}
	a.state.PlaybackEngine = "webview"
	p.failStart = true
	a.advanceLine("source failure")
	if a.now != "" || len(p.streams) != 1 || !strings.Contains(a.status, "找不到播放器") {
		t.Fatal("engine startup failure was treated as a route skip")
	}
}
