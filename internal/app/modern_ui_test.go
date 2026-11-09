package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestSettingsAndFavorites(t *testing.T) {
	a := largeChannelApp()
	a.path = filepath.Join(t.TempDir(), "state.json")
	tt := ui.NewTester(a.view, 1280, 820)
	if _, visible := tt.Find("保存设置"); visible {
		t.Fatal("settings should be collapsed initially")
	}
	if err := tt.Click("播放器设置"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("播放器"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("保存设置") {
		t.Fatal("player settings did not open")
	}
	if err := tt.Click("订阅源"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("顶部添加自定义源"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("添加并更新") || !tt.HasText("选择本地文件…") {
		t.Fatal("M3U imports should be part of settings")
	}
	if _, visible := tt.Find("视频画面"); visible {
		t.Fatal("video should not cover settings")
	}
	if err := tt.Click("返回播放"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"添加并更新", "选择本地文件…", "订阅地址或文件路径", "保存设置"} {
		if _, visible := tt.Find(label); visible {
			t.Fatalf("settings control %q leaked onto player page", label)
		}
	}
	contained(t, "视频画面", requiredRect(t, tt, "播放面板"), requiredRect(t, tt, "视频画面"))
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("我的收藏"); err != nil {
		t.Fatal(err)
	}
	if !a.favoritesOnly || !tt.HasText("1 个频道") {
		t.Fatal("favorites tab did not filter")
	}
	if err := tt.Click("全部频道"); err != nil {
		t.Fatal(err)
	}
	if a.favoritesOnly {
		t.Fatal("all channels tab did not reset filter")
	}
}

func TestImmersiveTitleBarInsets(t *testing.T) {
	a := largeChannelApp()
	tt := ui.NewTester(a.view, 1280, 820)
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
	bar := requiredRect(t, tt, "沉浸顶栏")
	badge := requiredRect(t, tt, "READY")
	if bar.Y != 0 || badge.X+badge.W > 1280-138 {
		t.Fatalf("native window controls overlap content: bar=%+v badge=%+v", bar, badge)
	}
	video := requiredRect(t, tt, "视频画面")
	if video.Y < 40 {
		t.Fatalf("native window controls overlap video: %+v", video)
	}
	if err := tt.Click("播放器设置"); err != nil {
		t.Fatal(err)
	}
	back := requiredRect(t, tt, "返回播放")
	if back.Y < 40 && back.X+back.W > 1280-138 {
		t.Fatalf("native controls overlap settings back button: %+v", back)
	}
}

func TestSoftNeutralNightSurfaces(t *testing.T) {
	for _, color := range []ui.Color{appTheme.Background, appTheme.Surface, appTheme.SurfaceHover, appTheme.SurfacePressed, appTheme.Border, appTheme.Text, appTheme.TextMuted, appTheme.Scrollbar, panelColor} {
		// Apple-style graphite has a subtle cool tint, not a saturated fill.
		if max(color.R, color.G, color.B)-min(color.R, color.G, color.B) > 10 {
			t.Fatalf("night surface is too saturated: %+v", color)
		}
		if color.A != 255 {
			t.Fatalf("fallback surface must remain opaque: %+v", color)
		}
	}
}

func TestPlaybackPreviousNextButtons(t *testing.T) {
	a, engine, _, ch := playbackFixture(t)
	a.group = "全部分组"
	a.state.Channels = []playlist.Channel{ch, {Name: "Other", URL: "https://example.org/other"}}
	tt := ui.NewTester(a.view, 1280, 820)
	for _, label := range []string{"上一频道", "下一频道"} {
		contained(t, label, requiredRect(t, tt, "播放控制栏"), requiredRect(t, tt, label))
	}
	if err := tt.Click("下一频道"); err != nil {
		t.Fatal(err)
	}
	if a.now != ch.Name {
		t.Fatal("next button did not start the first channel")
	}
	if err := tt.Click("下一频道"); err != nil {
		t.Fatal(err)
	}
	if a.now != "Other" {
		t.Fatal("next button did not advance")
	}
	if err := tt.Click("上一频道"); err != nil {
		t.Fatal(err)
	}
	if a.now != ch.Name || len(engine.streams) != 3 {
		t.Fatal("previous button did not return to the prior channel")
	}
}

func TestImmersivePreviousNextButtons(t *testing.T) {
	for _, windowMode := range []bool{false, true} {
		a, _, _, ch := playbackFixture(t)
		a.cinema, a.windowCinema = !windowMode, windowMode
		a.group = "全部分组"
		a.state.Channels = []playlist.Channel{ch, {Name: "Other", URL: "https://example.org/other"}}
		tt := ui.NewTester(a.view, 1000, 620)
		for _, label := range []string{"上一频道", "下一频道"} {
			contained(t, label, requiredRect(t, tt, "播放控制栏"), requiredRect(t, tt, label))
		}
		if err := tt.Click("下一频道"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("下一频道"); err != nil {
			t.Fatal(err)
		}
		if a.now != "Other" {
			t.Fatal("immersive next button did not advance")
		}
		if err := tt.Click("上一频道"); err != nil {
			t.Fatal(err)
		}
		if a.now != ch.Name {
			t.Fatal("immersive previous button did not go back")
		}
	}
}

// Optional visual artifact, containing synthetic channel data only.
func TestModernPreview(t *testing.T) {
	path := os.Getenv("LIUGUANG_TV_PREVIEW")
	if path == "" {
		t.Skip("set LIUGUANG_TV_PREVIEW to export a UI preview PNG")
	}
	a := largeChannelApp()
	names := []string{"中央综合", "新闻直播间", "城市生活", "自然与地理", "电影时光", "音乐现场", "体育精选", "纪录视界", "少儿天地", "文化之旅"}
	for i, name := range names {
		a.state.Channels[i].Name = name
		a.state.Channels[i].Group = "精选频道"
	}
	a.state.Favorites[a.state.Channels[1].URL] = true
	a.status = "准备就绪 · 从频道库选择一个频道开始观看"
	tt := ui.NewTester(a.view, 1280, 820)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
