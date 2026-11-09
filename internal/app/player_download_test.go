package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMPVDownloadOpensOfficialPageWithoutChangingPlayerSettings(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Run(tag, func(t *testing.T) {
			useTestLanguage(t, tag)
			previous := openPlayerExternal
			var opened string
			openPlayerExternal = func(target string) error { opened = target; return nil }
			t.Cleanup(func() { openPlayerExternal = previous })
			a := &application{showSettings: true, settingsPlayerTab: true, nowURL: "https://example.org/live", playerPathDraft: "unsaved-path", playerPathDraftLoaded: true}
			a.state.Player = "existing-path"
			tt := ui.NewTester(a.view, 1000, 620)
			contained(t, "mpv download", requiredRect(t, tt, tr("播放器配置")), requiredRect(t, tt, tr("下载 mpv")))
			if err := tt.Click(tr("下载 mpv")); err != nil {
				t.Fatal(err)
			}
			if opened != "https://mpv.io/installation/" {
				t.Fatalf("unexpected download URL: %q", opened)
			}
			if a.state.Player != "existing-path" || a.playerPathDraft != "unsaved-path" || a.nowURL != "https://example.org/live" {
				t.Fatal("opening the download page changed settings, draft or playback")
			}
			a.state.PlaybackEngine = "webview"
			tt.Frame()
			if _, found := tt.Find(tr("下载 mpv")); found {
				t.Fatal("mpv download action shown for WebView")
			}
		})
	}
}

func TestMPVDownloadErrorIsVisibleAndClearsOnSuccess(t *testing.T) {
	previous := openPlayerExternal
	t.Cleanup(func() { openPlayerExternal = previous })
	openPlayerExternal = func(string) error { return errors.New("browser launch failed") }
	a := &application{showSettings: true, settingsPlayerTab: true}
	a.openMPVDownload()
	message := a.settingsErrors["player-download"]
	if !strings.Contains(message, mpvDownloadURL) || !strings.Contains(message, "browser launch failed") {
		t.Fatalf("download error lost URL or diagnostic detail: %q", message)
	}
	tt := ui.NewTester(a.view, 1000, 620)
	requiredRect(t, tt, "设置错误 player-download")
	openPlayerExternal = func(string) error { return nil }
	a.openMPVDownload()
	if a.settingsErrors["player-download"] != "" {
		t.Fatal("successful retry left a stale download error")
	}
}
