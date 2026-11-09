package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"testing"
)

func TestPlaybackEngineDefaultAndSelectionStopsSession(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	if a.webPlayback() {
		t.Fatal("new state must default to mpv")
	}
	a.play(ch)
	a.setPlaybackEngine("webview")
	if a.now != "" || a.player != nil || !a.webPlayback() || a.startupTimer != nil || a.stallTimer != nil {
		t.Fatal("engine switch retained playback")
	}
	select {
	case <-p.streams[0].done:
	default:
		t.Fatal("old player survived switch")
	}
	if !a.supportsPlaybackControls() || !a.supportsPlaybackReady() {
		t.Fatal("WebView controls must be platform-independent")
	}
	a.setPlaybackEngine("mpv")
	if a.webPlayback() || a.state.PlaybackEngine != "mpv" {
		t.Fatal("mpv selection failed")
	}
}
func TestWebViewSettingsHideMPVPath(t *testing.T) {
	a := &application{showSettings: true, settingsPlayerTab: true, state: store.State{PlaybackEngine: "webview", Player: "preserve-path"}}
	tt := ui.NewTester(a.view, 1000, 620)
	requiredRect(t, tt, "播放引擎")
	requiredRect(t, tt, "选择 内置 WebView")
	if _, found := tt.Find("保存设置"); found {
		t.Fatal("WebView selection saves automatically; no path save button expected")
	}
	if _, found := tt.Find("mpv 路径"); found {
		t.Fatal("mpv path shown for WebView")
	}
	a.state.PlaybackEngine = "mpv"
	tt.Frame()
	requiredRect(t, tt, "mpv 路径")
	requiredRect(t, tt, "选择 mpv 程序")
	if a.state.Player != "preserve-path" {
		t.Fatal("engine switch discarded configured path")
	}
}
func TestWebViewUsesSharedReadinessAndBoundedFailover(t *testing.T) {
	a, p, queue, ch := playbackFixture(t)
	a.state.PlaybackEngine = "webview"
	a.play(ch)
	if a.startupTimer == nil {
		t.Fatal("WebView startup is not bounded")
	}
	p.streams[0].opt.OnReady()
	pumpPlayback(t, queue, func() bool { return a.lineReady })
	if a.stallTimer == nil {
		t.Fatal("WebView has no stall watchdog")
	}
	a.togglePause()
	if !a.controls.Paused {
		t.Fatal("WebView pause did not use shared controls")
	}
	a.controls = player.Controls{Volume: 70}
	a.lineTimedOut(a.generation - 1)
	if len(p.streams) != 1 {
		t.Fatal("stale timeout changed engine")
	}
	a.stop()
}
