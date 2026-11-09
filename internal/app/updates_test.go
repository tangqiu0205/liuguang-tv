package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUpdateActionOnlyInSettings(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Run(tag, func(t *testing.T) {
			useTestLanguage(t, tag)
			previous := checkForUpdates
			calls := 0
			checkForUpdates = func() { calls++ }
			t.Cleanup(func() { checkForUpdates = previous })
			a := largeChannelApp()
			tt := ui.NewTester(a.view, 1000, 620)
			if _, found := tt.Find(tr("检查更新")); found {
				t.Fatal("update action should not appear in the playback header")
			}
			a.showSettings = true
			a.settingsAboutTab = true
			tt.Frame()
			if err := tt.Click(tr("检查更新")); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("settings update action called updater %d times, want 1", calls)
			}
		})
	}
}
