package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestSettingsTabsStayAccessibleWithManySources(t *testing.T) {
	a := &application{showSettings: true, status: "准备就绪", state: store.State{}}
	for i := 0; i < 100; i++ {
		a.state.Subscriptions = append(a.state.Subscriptions, store.Subscription{ID: fmt.Sprint(i), Name: fmt.Sprintf("Source %d", i)})
	}
	tt := ui.NewTester(a.view, 1000, 620)
	if _, visible := tt.Find("播放器配置"); visible {
		t.Fatal("player settings should not be below source cards")
	}
	tabs := requiredRect(t, tt, "设置标签栏")
	list := requiredRect(t, tt, "设置列表")
	tt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, 2000)
	position := a.settingsScroll.Y
	if position <= 0 {
		t.Fatal("test did not scroll the source list")
	}
	if after := requiredRect(t, tt, "设置标签栏"); after != tabs {
		t.Fatal("settings tabs scrolled with subscriptions")
	}
	if _, visible := tt.Find("配置备份与迁移"); visible {
		t.Fatal("backup controls should not be below subscription cards")
	}
	if _, visible := tt.Find("EPG 设置"); visible {
		t.Fatal("EPG settings should not be below source cards")
	}
	if err := tt.Click("EPG 节目单"); err != nil {
		t.Fatal(err)
	}
	if !a.settingsEPGTab {
		t.Fatal("EPG tab did not activate")
	}
	contained(t, "EPG 空状态", requiredRect(t, tt, "EPG 页面"), requiredRect(t, tt, "EPG 空状态"))
	if !tt.HasText("尚未添加节目单 EPG") {
		t.Fatal("EPG empty state is missing")
	}
	if _, visible := tt.Find("订阅卡片网格"); visible {
		t.Fatal("source cards leaked onto EPG tab")
	}
	if err := tt.Click("备份"); err != nil {
		t.Fatal(err)
	}
	if !a.settingsBackupTab || a.settingsEPGTab || a.settingsPlayerTab || a.settingsAppearanceTab {
		t.Fatal("backup tab did not activate exclusively")
	}
	for _, label := range []string{"导出配置…", "导入配置…"} {
		contained(t, label, requiredRect(t, tt, "备份页面"), requiredRect(t, tt, label))
	}
	if _, visible := tt.Find("设置列表"); visible {
		t.Fatal("subscription list leaked onto backup tab")
	}
	if err := tt.Click("播放器"); err != nil {
		t.Fatal(err)
	}
	if !a.settingsPlayerTab || a.settingsBackupTab {
		t.Fatal("player tab did not activate")
	}
	for _, label := range []string{"mpv 路径", "保存设置"} {
		r := requiredRect(t, tt, label)
		contained(t, label, requiredRect(t, tt, "播放器页面"), r)
		if r.Y+r.H > 620 {
			t.Fatalf("player control %s is below the visible window: %+v", label, r)
		}
	}
	if _, visible := tt.Find("订阅卡片网格"); visible {
		t.Fatal("source cards leaked onto player tab")
	}
	if err := tt.Click("订阅源"); err != nil {
		t.Fatal(err)
	}
	// Source scroll position is independent of the player's scroll container.
	if a.settingsScroll.Y != position {
		t.Fatalf("subscription scroll position was lost: before=%v after=%v", position, a.settingsScroll.Y)
	}
}
func TestEPGTabPreservesScrollAndDraft(t *testing.T) {
	a := &application{showSettings: true}
	a.epg.jobs = make(map[string]*epgJob)
	for i := 0; i < 30; i++ {
		source := fmt.Sprintf("https://example.org/%d.xml", i)
		a.state.EPGSources = append(a.state.EPGSources, store.EPGSourceEntry{ID: fmt.Sprint(i), Name: fmt.Sprint(i), Source: source})
		a.epg.jobs[source] = &epgJob{nextTry: time.Now().Add(time.Hour)}
	}
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("EPG 节目单"); err != nil {
		t.Fatal(err)
	}
	a.source = "https://example.org/draft.xml"
	tt.Frame()
	page := requiredRect(t, tt, "EPG 页面")
	tt.Scroll(page.X+page.W/2, page.Y+page.H/2, 0, 1200)
	position := a.epgSettingsScroll.Y
	if position <= 0 {
		t.Fatal("test did not scroll the EPG page")
	}
	if err := tt.Click("订阅源"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("EPG 节目单"); err != nil {
		t.Fatal(err)
	}
	if a.epgSettingsScroll.Y != position || a.source != "https://example.org/draft.xml" {
		t.Fatal("EPG scroll position or draft was lost on tab switch")
	}
}

func TestBackupTabLayoutAndIsolation(t *testing.T) {
	for _, scale := range []float32{1, 1.5} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			a := &application{showSettings: true, status: "准备就绪", lastImportSnapshot: "state-before-import-example.json"}
			tt := ui.NewTester(a.view, int(1000*scale), int(620*scale))
			tt.SetScale(scale)
			for _, tab := range []string{"订阅源", "播放器", "外观", "备份", "关于"} {
				contained(t, tab, requiredRect(t, tt, "设置标签栏"), requiredRect(t, tt, tab))
			}
			for _, tab := range []string{"订阅管理", "EPG 节目单"} {
				contained(t, tab, requiredRect(t, tt, "订阅源子标签栏"), requiredRect(t, tt, tab))
			}
			if err := tt.Click("备份"); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{"导出配置…", "导入配置…"} {
				contained(t, label, requiredRect(t, tt, "备份页面"), requiredRect(t, tt, label))
			}
			for _, absent := range []string{"订阅卡片网格", "订阅编辑栏", "播放器配置", "外观页面"} {
				if _, found := tt.Find(absent); found {
					t.Fatalf("%s leaked onto backup tab", absent)
				}
			}
			if tt.HasText("最近一次导入前恢复快照：" + a.lastImportSnapshot) {
				t.Fatal("recovery detail should be collapsed initially")
			}
			if err := tt.Click("日志、快捷键与恢复说明"); err != nil {
				t.Fatal(err)
			}
			tt.Advance(250 * time.Millisecond)
			if !tt.HasText("最近一次导入前恢复快照：" + a.lastImportSnapshot) {
				t.Fatal("recovery snapshot hint missing")
			}
			for _, tab := range []string{"播放器", "外观", "关于", "订阅源"} {
				if err := tt.Click(tab); err != nil {
					t.Fatal(err)
				}
				if _, found := tt.Find("备份页面"); found {
					t.Fatalf("backup page leaked onto %s tab", tab)
				}
			}
		})
	}
}

func TestSettingsSidebarUsesThemePanelColor(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			a := &application{showSettings: true, state: store.State{ThemeMode: mode}}
			tt := ui.NewTester(a.view, 1000, 620)
			sidebar := requiredRect(t, tt, "设置标签栏")
			pixel := tt.Image().RGBAAt(int(sidebar.X+sidebar.W/2), int(sidebar.Y+sidebar.H-20))
			want := panelBackground(themeForMode(mode, false))
			if pixel.R != want.R || pixel.G != want.G || pixel.B != want.B {
				t.Fatalf("settings sidebar does not follow theme: got=%v want=%v", pixel, want)
			}
		})
	}
}

func TestSettingsSidebarAndAddSourceAction(t *testing.T) {
	a := &application{showSettings: true}
	tt := ui.NewTester(a.view, 1000, 620)
	sidebar := requiredRect(t, tt, "设置标签栏")
	contained(t, "设置应用图标", sidebar, requiredRect(t, tt, "设置应用图标"))
	content := requiredRect(t, tt, "设置内容区")
	title := requiredRect(t, tt, "设置标题")
	description := requiredRect(t, tt, "设置说明")
	if title.X != description.X || description.Y < title.Y+title.H {
		t.Fatal("settings title and description are not aligned")
	}
	contained(t, "设置说明", requiredRect(t, tt, "设置顶栏"), description)
	if sidebar.X+sidebar.W > content.X {
		t.Fatal("settings sidebar overlaps the content")
	}
	if err := tt.Click("顶部添加自定义源"); err != nil {
		t.Fatal(err)
	}
	if !a.showSubEditor {
		t.Fatal("toolbar add action did not open the subscription editor")
	}
	if _, visible := tt.Find("订阅编辑栏"); !visible {
		t.Fatal("subscription editor is not visible")
	}
}

func TestSettingsHeaderPlacesReturnBelowCaptionControls(t *testing.T) {
	a := &application{showSettings: true}
	tt := ui.NewTester(a.view, 1280, 820)
	icon := requiredRect(t, tt, "设置标题图标")
	tt.SetTitleBar(ui.TitleBar{Height: 32, Right: 140})
	if after := requiredRect(t, tt, "设置标题图标"); after.X != icon.X {
		t.Fatalf("right caption inset moved title: before=%+v after=%+v", icon, after)
	}
	title := requiredRect(t, tt, "设置标题")
	description := requiredRect(t, tt, "设置说明")
	if title.X != description.X {
		t.Fatal("settings title and description are not aligned")
	}
	button := requiredRect(t, tt, "返回播放")
	header := requiredRect(t, tt, "设置顶栏")
	if button.Y < 32 || button.X+button.W != header.X+header.W {
		t.Fatal("return button should be right-aligned below caption controls")
	}
}

func TestSettingsTabSwitchPreservesEditorDraft(t *testing.T) {
	a := &application{showSettings: true}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("顶部添加自定义源"); err != nil {
		t.Fatal(err)
	}
	a.subName = "Unfinished source"
	a.source = "https://example.org/draft.m3u"
	tt.Frame()
	for _, tab := range []string{"EPG 节目单", "播放器", "外观", "备份", "关于"} {
		if err := tt.Click(tab); err != nil {
			t.Fatal(err)
		}
		if _, visible := tt.Find("订阅编辑栏"); visible {
			t.Fatalf("subscription editor should be hidden on %s tab", tab)
		}
	}
	if err := tt.Click("订阅源"); err != nil {
		t.Fatal(err)
	}
	if _, visible := tt.Find("订阅编辑栏"); !visible {
		t.Fatal("subscription editor did not restore")
	}
	if a.subName != "Unfinished source" || a.source != "https://example.org/draft.m3u" {
		t.Fatal("tab switch discarded editor draft")
	}
}
