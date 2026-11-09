package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWindowCinemaUsesFullscreenLayout(t *testing.T) {
	a := largeChannelApp()
	tt := ui.NewTester(a.view, 1000, 620)
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
	normal := requiredRect(t, tt, "视频画面")
	if err := tt.Click("窗口全屏"); err != nil {
		t.Fatal(err)
	}
	if !a.windowCinema || a.cinema {
		t.Fatal("window cinema must not enter OS fullscreen")
	}
	if _, visible := tt.Find("频道面板"); visible {
		t.Fatal("channel library should be hidden")
	}
	video := requiredRect(t, tt, "视频画面")
	if video.X != 0 || video.W != 1000 || video.W <= normal.W || video.Y != 0 {
		t.Fatalf("window cinema should fill window without a header: %+v", video)
	}
	assertNoFullscreenHeader(t, tt)
	bar := requiredRect(t, tt, "播放控制栏")
	if bar.W != 760 || bar.H != 56 || bar.X != 120 || bar.Y+bar.H != 604 {
		t.Fatalf("window cinema should use the centered fullscreen controls: %+v", bar)
	}
	contained(t, "窗口全屏按钮", requiredRect(t, tt, "播放控制栏"), requiredRect(t, tt, "退出窗口全屏"))
	tt.Key(0, ui.KeyEscape)
	if a.windowCinema || a.cinema {
		t.Fatal("Escape did not restore standard layout")
	}
	if _, visible := tt.Find("频道面板"); !visible {
		t.Fatal("library was not restored")
	}
}
func TestWindowAndOSFullscreenTransitions(t *testing.T) {
	a := largeChannelApp()
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("窗口全屏"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("全屏"); err != nil {
		t.Fatal(err)
	}
	if !a.cinema || !a.windowCinema {
		t.Fatal("OS fullscreen should retain the previous window mode")
	}
	if _, visible := tt.Find("窗口模式标题"); visible {
		t.Fatal("OS fullscreen should not reserve window controls")
	}
	tt.Key(0, ui.KeyEscape)
	if a.cinema || !a.windowCinema {
		t.Fatal("leaving OS fullscreen did not restore window cinema")
	}
	if err := tt.Click("全屏"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("窗口全屏"); err != nil {
		t.Fatal(err)
	}
	if a.cinema || !a.windowCinema {
		t.Fatal("window-fullscreen button did not leave OS fullscreen")
	}
	if err := tt.Click("退出窗口全屏"); err != nil {
		t.Fatal(err)
	}
	if a.windowCinema || a.cinema {
		t.Fatal("exit window cinema did not restore library layout")
	}
}
