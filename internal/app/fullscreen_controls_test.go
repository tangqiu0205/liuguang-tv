package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestFullscreenControlsPointerAndHold(t *testing.T) {
	var s fullscreenControls
	start := time.Unix(100, 0)
	if !s.update(start, 10, 20, true, false) {
		t.Fatal("controls should initially be visible")
	}
	if s.update(start.Add(3*time.Second), 10, 20, true, false) {
		t.Fatal("stationary pointer should hide controls")
	}
	if !s.update(start.Add(4*time.Second), 11, 20, true, false) {
		t.Fatal("moving pointer should reveal controls")
	}
	if !s.update(start.Add(8*time.Second), 11, 20, true, true) {
		t.Fatal("open panel should keep controls visible")
	}
	if !s.update(start.Add(9*time.Second), 0, 0, false, false) {
		t.Fatal("hold should extend visibility after panel closes")
	}
	if s.update(start.Add(12*time.Second), 0, 0, false, false) {
		t.Fatal("controls should hide after pointer leaves")
	}
	if !s.update(start.Add(13*time.Second), 11, 20, true, false) {
		t.Fatal("pointer re-entry should reveal controls")
	}
}

func TestFullscreenControlsLayoutAndRestore(t *testing.T) {
	a := largeChannelApp()
	a.now = "test channel"
	x := int32(100)
	a.fullscreenPointer = func() (int32, int32, bool, error) { return x, 100, true, nil }
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("全屏"); err != nil {
		t.Fatal(err)
	}
	assertNoFullscreenHeader(t, tt)
	requiredRect(t, tt, "播放控制栏")
	if fullscreenPointerSupported {
		video := requiredRect(t, tt, "视频画面")
		if video.X != 0 || video.Y != 0 || video.W != 1280 || video.H != 820 {
			t.Fatalf("visible controls must not resize video: %+v", video)
		}
	}
	pixel := tt.Image().RGBAAt(5, 5)
	if pixel.R > 50 || pixel.G > 50 || pixel.B > 50 {
		t.Fatalf("fullscreen video should retain its dark background: %v", pixel)
	}
	tt.Move(640, 780)
	a.fullscreenUI.until = time.Now().Add(-time.Second)
	tt.Frame()
	if _, ok := tt.Find("播放控制栏"); !ok {
		t.Fatal("controls must remain visible while the pointer is over the bottom bar")
	}
	tt.Move(600, 400)
	a.fullscreenUI.until = time.Now().Add(-time.Second)
	tt.Frame()
	video := requiredRect(t, tt, "视频画面")
	if video.X != 0 || video.Y != 0 || video.W != 1280 || video.H != 820 {
		t.Fatalf("video does not fill fullscreen: %+v", video)
	}
	if _, ok := tt.Find("播放控制栏"); ok {
		t.Fatal("controls remain visible")
	}
	if _, ok := tt.Find("全屏节目顶栏"); ok {
		t.Fatal("header remains visible")
	}
	x++
	tt.Frame()
	requiredRect(t, tt, "播放控制栏")
	assertNoFullscreenHeader(t, tt)
	if err := tt.Click("退出全屏"); err != nil {
		t.Fatal(err)
	}
	if a.cinema {
		t.Fatal("fullscreen did not exit")
	}
}

func TestWindowCinemaControlsAutoHideAndRestore(t *testing.T) {
	a := largeChannelApp()
	a.windowCinema, a.now = true, "test channel"
	x := int32(100)
	a.fullscreenPointer = func() (int32, int32, bool, error) { return x, 100, true, nil }
	tt := ui.NewTester(a.view, 1280, 820)
	requiredRect(t, tt, "播放控制栏")
	assertNoFullscreenHeader(t, tt)
	a.fullscreenUI.until = time.Now().Add(-time.Second)
	tt.Frame()
	if _, ok := tt.Find("播放控制栏"); ok {
		t.Fatal("window cinema controls must auto-hide")
	}
	video := requiredRect(t, tt, "视频画面")
	if video.X != 0 || video.Y != 0 || video.W != 1280 || video.H != 820 {
		t.Fatalf("window cinema video must fill the window: %+v", video)
	}
	x++
	tt.Frame()
	requiredRect(t, tt, "播放控制栏")
	assertNoFullscreenHeader(t, tt)
	if err := tt.Click("退出窗口全屏"); err != nil {
		t.Fatal(err)
	}
	requiredRect(t, tt, "频道面板")
}

func TestFullscreenControlsAreCompactAndCentered(t *testing.T) {
	for _, width := range []float32{560, 1000, 1280, 1920} {
		a := largeChannelApp()
		a.cinema, a.now = true, "test channel"
		tt := ui.NewTester(a.view, int(width), 820)
		bar := requiredRect(t, tt, "播放控制栏")
		panel := requiredRect(t, tt, "播放面板")
		wantWidth := min(float32(760), width-32)
		if bar.W != wantWidth || bar.H != 56 {
			t.Fatalf("width=%v unexpected control bar: %+v", width, bar)
		}
		if bar.X+bar.W/2 != panel.X+panel.W/2 || bar.Y+bar.H != panel.Y+panel.H-16 {
			t.Fatalf("width=%v bar is not centered above bottom: bar=%+v panel=%+v", width, bar, panel)
		}
		for _, label := range []string{"暂停", "上一频道", "下一频道", "停止", "窗口全屏", "退出全屏"} {
			contained(t, label, bar, requiredRect(t, tt, label))
		}
		assertNoFullscreenHeader(t, tt)
	}
}

func assertNoFullscreenHeader(t *testing.T, tt *ui.Tester) {
	t.Helper()
	for _, label := range []string{"全屏节目顶栏", "沉浸顶栏", "窗口模式标题", "节目摘要"} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("fullscreen must not show %s", label)
		}
	}
}

func TestFullscreenControlsPointerErrorKeepsExitAvailable(t *testing.T) {
	a := &application{cinema: true, now: "test", fullscreenPointer: func() (int32, int32, bool, error) { return 0, 0, false, errors.New("pointer failure") }}
	if !a.fullscreenBarsVisible(time.Now()) || a.status != "pointer failure" {
		t.Fatal("pointer failure must keep controls available and report error")
	}
}
