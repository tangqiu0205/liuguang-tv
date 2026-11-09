package app

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestThemeDefaultsFollowSystemAndOverridesStayFixed(t *testing.T) {
	a := &application{}
	var effective *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) { a.view(c); effective = c.Theme() }, 1000, 620)
	for _, tc := range []struct {
		mode                 string
		systemDark, wantDark bool
	}{
		{"", false, false}, {"", true, true}, {"system", false, false}, {"system", true, true},
		{"light", true, false}, {"dark", false, true}, {"unknown-old-value", true, true}, {"unknown-old-value", false, false},
	} {
		a.state.ThemeMode = tc.mode
		tt.SetDark(tc.systemDark)
		tt.Frame()
		if effective.Dark != tc.wantDark {
			t.Fatalf("mode=%q systemDark=%v got=%v", tc.mode, tc.systemDark, effective.Dark)
		}
		expected := dayPalette.Panel
		if tc.wantDark {
			expected = nightPalette.Panel
		}
		if panelBackground(effective) != expected {
			t.Fatal("panel palette did not follow theme")
		}
	}
	for _, tc := range []struct {
		mode   string
		source mygo.ThemeSource
	}{
		{"", mygo.ThemeSystem}, {"system", mygo.ThemeSystem}, {"light", mygo.ThemeLight}, {"dark", mygo.ThemeDark}, {"invalid", mygo.ThemeSystem},
	} {
		if nativeThemeSource(tc.mode) != tc.source {
			t.Fatal("native caption/menu appearance source mismatch")
		}
	}
}
func TestThemeSettingsApplyImmediatelyAndPersist(t *testing.T) {
	a := &application{showSettings: true, path: filepath.Join(t.TempDir(), "state.json")}
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	page := requiredRect(t, tt, "外观页面")
	for _, label := range []string{"跟随系统", "日间", "夜间"} {
		contained(t, label, page, requiredRect(t, tt, label))
	}
	for _, choice := range []struct{ label, mode, effective string }{{"夜间", "dark", "夜间"}, {"日间", "light", "日间"}, {"主题选项 跟随系统", "system", "夜间"}} {
		if choice.mode == "system" {
			tt.SetDark(true)
		}
		if err := tt.Click(choice.label); err != nil {
			t.Fatal(err)
		}
		if a.state.ThemeMode != choice.mode || !tt.HasText("当前显示："+choice.effective) {
			t.Fatalf("theme choice did not apply: %+v", choice)
		}
		loaded, err := store.Load(a.path)
		if err != nil || loaded.ThemeMode != choice.mode {
			t.Fatalf("theme choice not persisted: %v", err)
		}
	}
	tt.SetDark(false)
	if !tt.HasText("当前显示：日间") {
		t.Fatal("system change did not update system-following mode")
	}
}
func TestAppearanceTabPreservesSubscriptionDraft(t *testing.T) {
	a := &application{showSettings: true}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("顶部添加自定义源"); err != nil {
		t.Fatal(err)
	}
	a.subName, a.source = "Draft", "https://example.com/draft.m3u"
	tt.Frame()
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("订阅编辑栏"); ok {
		t.Fatal("subscription editor leaked onto appearance page")
	}
	if err := tt.Click("播放器"); err != nil {
		t.Fatal(err)
	}
	if !a.settingsPlayerTab || a.settingsAppearanceTab {
		t.Fatal("appearance and player tabs overlap")
	}
	if err := tt.Click("订阅源"); err != nil {
		t.Fatal(err)
	}
	requiredRect(t, tt, "订阅编辑栏")
	if a.subName != "Draft" || a.source != "https://example.com/draft.m3u" {
		t.Fatal("appearance tab discarded subscription draft")
	}
}
func TestThemeChangesDoNotRestartPlayback(t *testing.T) {
	a, engine, _, ch := playbackFixture(t)
	a.play(ch)
	generation := a.generation
	for _, mode := range []string{"dark", "light", "system"} {
		a.setThemeMode(mode)
	}
	if len(engine.streams) != 1 || a.generation != generation || a.nowURL != ch.URL {
		t.Fatal("theme change restarted or altered playback")
	}
	select {
	case <-engine.streams[0].done:
		t.Fatal("theme change stopped player")
	default:
	}
}
func luminance(c ui.Color) float64 {
	channel := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return .2126*channel(c.R) + .7152*channel(c.G) + .0722*channel(c.B)
}
func TestChannelFilterSelectedTabHasOutlinedSelection(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			a := &application{state: store.State{ThemeMode: mode}, group: "全部分组"}
			tt := ui.NewTester(a.view, 1000, 620)
			theme := themeForMode(mode, false)
			background, accent := selectedBackground(theme), theme.Accent
			for _, label := range []string{"全部频道", "我的收藏", "全部频道"} {
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				r := requiredRect(t, tt, label)
				pixel := tt.Image().RGBAAt(int(r.X+8), int(r.Y+r.H/2))
				if pixel.R != background.R || pixel.G != background.G || pixel.B != background.B {
					t.Fatalf("selected tab %s does not have tinted selection background: %+v", label, pixel)
				}
				// Fractional flex widths antialias a one-DIP border across pixels.
				distance := func(r, g, b uint8) float64 {
					return math.Abs(float64(r)-float64(accent.R)) + math.Abs(float64(g)-float64(accent.G)) + math.Abs(float64(b)-float64(accent.B))
				}
				borderVisible := false
				for dx := 0; dx < 3; dx++ {
					edge := tt.Image().RGBAAt(int(r.X)+dx, int(r.Y+r.H/2))
					if distance(edge.R, edge.G, edge.B) < distance(background.R, background.G, background.B)*.8 {
						borderVisible = true
					}
				}
				if !borderVisible {
					t.Fatalf("selected tab %s lacks accent border", label)
				}
			}
		})
	}
}

func TestThemeTextContrast(t *testing.T) {
	for _, theme := range []*ui.Theme{appTheme, lightAppTheme} {
		palette := paletteFor(theme)
		for _, pair := range [][2]ui.Color{{theme.Text, palette.Panel}, {theme.TextMuted, palette.Panel}, {theme.Text, palette.Card}, {theme.Accent, palette.Selected}, {theme.AccentText, theme.Accent}} {
			a, b := luminance(pair[0]), luminance(pair[1])
			if a < b {
				a, b = b, a
			}
			if ratio := (a + .05) / (b + .05); ratio < 4.5 {
				t.Errorf("dark=%v insufficient text contrast %.2f: %+v", theme.Dark, ratio, pair)
			}
		}
	}
}
