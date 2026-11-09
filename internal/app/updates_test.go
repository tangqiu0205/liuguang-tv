package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMainWindowCheckForUpdates(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Run(tag, func(t *testing.T) {
			useTestLanguage(t, tag)
			previous := checkForUpdates
			calls := 0
			checkForUpdates = func() { calls++ }
			t.Cleanup(func() { checkForUpdates = previous })
			a := largeChannelApp()
			tt := ui.NewTester(a.view, 1000, 620)
			header := requiredRect(t, tt, tr("沉浸顶栏"))
			button := requiredRect(t, tt, tr("检查更新"))
			contained(t, "update action", header, button)
			if err := tt.Click(tr("检查更新")); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || a.showSettings || a.nowURL != "" {
				t.Fatal("update action did not open the updater independently of playback/settings")
			}
			// The immersive view keeps its header hidden; update controls must
			// not float over the video or interfere with fullscreen controls.
			a.cinema = true
			tt.Frame()
			if _, found := tt.Find(tr("检查更新")); found {
				t.Fatal("update action leaked onto the fullscreen video")
			}
		})
	}
}
