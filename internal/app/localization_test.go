package app

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/l10n"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestMain(m *testing.M) {
	// Existing UI assertions use the source language, independently of the
	// developer's system language or saved desktop preferences.
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	os.Exit(m.Run())
}

func useTestLanguage(t *testing.T, tag string) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) })
}

func TestEnglishMainViewPreservesChannelData(t *testing.T) {
	useTestLanguage(t, "en")
	a := largeChannelApp()
	tt := ui.NewTester(a.view, 1000, 620)
	for _, text := range []string{"All channels", "Favorites", "All groups", "Channels: 1000", "Video playback area", "频道 0001", "测试分组"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q", text)
		}
	}
	if err := tt.Click("Channel group"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Channel group option 测试分组"); err != nil {
		t.Fatal(err)
	}
	if a.group != "测试分组" || a.filteredChannels().Len() != 1000 {
		t.Fatal("language changed group identity or filtering")
	}
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	tt.Type("0001")
	if a.filteredChannels().Len() != 1 {
		t.Fatal("English search did not filter channels")
	}
}

func TestLanguageSwitchInAppearanceSettings(t *testing.T) {
	a := largeChannelApp()
	a.showSettings = true
	a.settingsAppearanceTab = true
	a.source, a.subName = "https://example.org/list.m3u", "我的订阅"
	a.nowURL = "https://example.org/live"
	old := changeAppLocale
	oldRead := loadAppLanguagePreference
	var saved string
	loadAppLanguagePreference = func(string) (string, error) { return saved, nil }
	var selected []string
	changeAppLocale = func(tag string) {
		selected = append(selected, tag)
		saved = tag
		if tag == "" {
			tag = "zh-Hans"
		}
		i18n.SetLocaleFunc(func() string { return tag })
	}
	t.Cleanup(func() {
		changeAppLocale = old
		loadAppLanguagePreference = oldRead
		i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	})
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("English"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Interface language") || !tt.HasText("Appearance") {
		t.Fatal("language did not change immediately")
	}
	if a.source != "https://example.org/list.m3u" || a.subName != "我的订阅" || a.nowURL != "https://example.org/live" {
		t.Fatal("language switch changed editor data or playback")
	}
	if err := tt.Click("简体中文"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("界面语言") {
		t.Fatal("did not switch back to Chinese")
	}
	if err := tt.Click("语言选项 跟随系统"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(selected, ",") != "en,zh-Hans," {
		t.Fatalf("locale preferences were not delegated to MyGo: %v", selected)
	}
}

func TestLanguagePreferenceSelectionDiffersFromEffectiveLocale(t *testing.T) {
	useTestLanguage(t, "en")
	for _, preference := range []string{"", "en", "zh-Hans"} {
		t.Run("preference="+preference, func(t *testing.T) {
			a := &application{languagePreference: preference}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(lightAppTheme)
				a.languageSettings(c)
			}, 900, 340)
			if !tt.HasText("Current language: English") {
				t.Fatalf("missing effective language: %v", tt.Texts())
			}
			var first ui.Rect
			for i, option := range []struct{ tag, label string }{{"", "System"}, {"zh-Hans", "简体中文"}, {"en", "English"}} {
				r := requiredRect(t, tt, "Language option "+option.label)
				if i == 0 {
					first = r
				} else if r.Y != first.Y || r.W-first.W > 1 || first.W-r.W > 1 {
					t.Fatalf("language choices are not an equal-width row: %+v / %+v", first, r)
				}
				want := lightAppTheme.Surface
				if preference == option.tag {
					want = selectedBackground(lightAppTheme)
				}
				pixel := tt.Image().RGBAAt(int(r.X+8), int(r.Y+r.H/2))
				if pixel.R != want.R || pixel.G != want.G || pixel.B != want.B {
					t.Fatalf("wrong highlight for %q with saved preference %q: %+v", option.tag, preference, pixel)
				}
			}
		})
	}
}

func TestLanguagePreferenceReadErrorIsVisible(t *testing.T) {
	old := loadAppLanguagePreference
	loadAppLanguagePreference = func(string) (string, error) { return "", fmt.Errorf("permission denied") }
	t.Cleanup(func() { loadAppLanguagePreference = old })
	a := &application{languagePreference: "en"}
	a.refreshLanguagePreference()
	if a.languagePreference != "en" || !strings.Contains(a.settingsErrors["language"], "permission denied") {
		t.Fatal("preference read failure was disguised as system-following mode")
	}
	tt := ui.NewTester(a.languageSettings, 900, 340)
	if !tt.HasText("设置错误 language") {
		t.Fatalf("preference read failure was not displayed: %v", tt.Texts())
	}
}

func TestLanguagePreferenceSaveFailureIsVisible(t *testing.T) {
	oldChange, oldRead := changeAppLocale, loadAppLanguagePreference
	changeAppLocale = func(string) {}
	loadAppLanguagePreference = func(string) (string, error) { return "", nil }
	t.Cleanup(func() {
		changeAppLocale, loadAppLanguagePreference = oldChange, oldRead
	})
	a := &application{}
	a.setLanguage("en")
	if a.languagePreference != "" || a.settingsErrors["language"] == "" {
		t.Fatal("failed language persistence reported as successful selection")
	}
}

func TestEnglishSettingsPagesLayout(t *testing.T) {
	useTestLanguage(t, "en")
	for _, dark := range []bool{false, true} {
		for _, size := range [][2]float32{{1000, 620}, {1280, 820}} {
			t.Run(fmt.Sprint(dark, size), func(t *testing.T) {
				a := largeChannelApp()
				a.showSettings = true
				if dark {
					a.state.ThemeMode = "dark"
				} else {
					a.state.ThemeMode = "light"
				}
				tt := ui.NewTester(a.view, int(size[0]), int(size[1]))
				for _, page := range []string{"Player", "Appearance", "Backup", "About", "Subscriptions"} {
					if err := tt.Click(page); err != nil {
						t.Fatal(err)
					}
					content := requiredRect(t, tt, "Settings content")
					back := requiredRect(t, tt, "Back to player")
					if content.W <= 0 || content.H <= 0 || back.X+back.W > size[0] {
						t.Fatalf("invalid layout for %s: content=%+v back=%+v", page, content, back)
					}
				}
			})
		}
	}
}

func TestEnglishStaticPagesHaveNoUntranslatedChinese(t *testing.T) {
	useTestLanguage(t, "en")
	a := &application{group: "全部分组"}
	tt := ui.NewTester(a.view, 1280, 820)
	check := func() {
		t.Helper()
		for _, text := range tt.Texts() {
			if text != "简体中文" && text != "Language option 简体中文" && strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Errorf("untranslated application text: %q", text)
			}
		}
	}
	check()
	if err := tt.Click("Player settings"); err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"Player", "Appearance", "Backup", "About", "Subscriptions"} {
		if err := tt.Click(page); err != nil {
			t.Fatal(err)
		}
		check()
		if page == "Player" {
			if err := tt.Click("Compatibility notes"); err != nil {
				t.Fatal(err)
			}
			check()
		}
	}
	if err := tt.Click("EPG guide"); err != nil {
		t.Fatal(err)
	}
	check()
	if err := tt.Click("Add custom source from header"); err != nil {
		t.Fatal(err)
	}
	check()
}

func TestEnglishBackgroundSaveFeedback(t *testing.T) {
	useTestLanguage(t, "en")
	a := &application{status: tr("播放器路径已提交保存，下次播放生效"), pendingSettingsSuccess: tr("播放器路径已保存，下次播放生效")}
	a.finishSettingsSave()
	if strings.Contains(a.status, "queued for saving") || a.settingsToast != "Player path saved; applies to the next playback" {
		t.Fatalf("incorrect save feedback: %q / %q", a.status, a.settingsToast)
	}
	if label, _ := statusStyle("Failed to save settings: disk full", lightAppTheme); label != "Error" {
		t.Fatalf("English failure misclassified: %q", label)
	}
	// A refresh completing after a language change must not remain pending.
	a.status = l10n.In("en", "订阅更新完成，正在整理频道…")
	a.refreshResult = subscriptionRefreshResult{completed: 1, name: "测试源"}
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	a.finishRefreshStatus()
	if a.status != "订阅 测试源 更新完成" {
		t.Fatalf("refresh stalled after locale change: %q", a.status)
	}
}

func TestOnlyBuiltInGroupAndRouteLabelsAreTranslated(t *testing.T) {
	useTestLanguage(t, "en")
	if groupTitle("全部分组") != "All groups" || groupTitle("我的分组") != "我的分组" {
		t.Fatal("group translation changed source data")
	}
	ch := playlist.Channel{URL: "https://example.org/live", Lines: []playlist.Line{{URL: "https://example.org/live", Label: "高清"}}}
	if got := lineLabels(ch); len(got) != 1 || got[0] != "Route 1 · 高清" {
		t.Fatalf("route label translation changed source data: %v", got)
	}
}
