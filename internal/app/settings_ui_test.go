package app

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestSettingsPagesLayoutAndAboutIsolation(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		for _, scale := range []float32{1, 1.5} {
			t.Run(fmt.Sprintf("%s/%v", mode, scale), func(t *testing.T) {
				a := &application{showSettings: true, status: "准备就绪", version: "9.8.7", path: filepath.Join(t.TempDir(), "state.json"), state: store.State{ThemeMode: mode}}
				tt := ui.NewTester(a.view, int(1000*scale), int(620*scale))
				tt.SetScale(scale)
				for _, tab := range []struct{ name, page string }{{"播放器", "播放器页面"}, {"备份", "备份页面"}, {"关于", "关于页面"}, {"外观", "外观页面"}, {"订阅源", "设置列表"}, {"EPG 节目单", "EPG 页面"}} {
					if err := tt.Click(tab.name); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"订阅源", "播放器", "外观", "备份", "关于"} {
						contained(t, name, requiredRect(t, tt, "设置标签栏"), requiredRect(t, tt, name))
					}
					page := requiredRect(t, tt, tab.page)
					if tab.name == "播放器" {
						for _, name := range []string{"选择 mpv 播放器（默认）", "选择 内置 WebView", "mpv 路径", "保存设置"} {
							contained(t, name, page, requiredRect(t, tt, name))
						}
						if a.playerCompatibilityOpen {
							t.Fatal("compatibility should be collapsed initially")
						}
					}
					if tab.name == "备份" {
						for _, name := range []string{"导出配置…", "导入配置…", "导出备份区域", "恢复配置区域"} {
							contained(t, name, page, requiredRect(t, tt, name))
						}
						if !tt.HasText("整份替换，不合并；成功后停止播放。") {
							t.Fatal("restore risk missing")
						}
					}
					if tab.name == "关于" {
						if !a.settingsAboutTab || a.settingsPlayerTab || a.settingsAppearanceTab || a.settingsBackupTab {
							t.Fatal("about tab is not exclusive")
						}
						for _, name := range []string{"应用版本", "检查更新", "项目主页", "问题反馈", "内容与隐私说明"} {
							contained(t, name, page, requiredRect(t, tt, name))
						}
						for _, text := range []string{"技术与组件", "运行平台：", "原生界面：", "可选 WebView 播放：", projectURL} {
							if tt.HasText(text) {
								t.Fatalf("removed technical detail is still visible: %s", text)
							}
						}
						if !tt.HasText("版本：9.8.7") {
							t.Fatal("about version missing")
						}
						for _, name := range []string{"播放器配置", "订阅编辑栏", "配置备份与迁移"} {
							if _, found := tt.Find(name); found {
								t.Fatalf("%s leaked into about", name)
							}
						}
					}
					if dir := os.Getenv("LIUGUANG_TV_SETTINGS_PREVIEW"); dir != "" && scale == 1 && (tab.name == "播放器" || tab.name == "备份" || tab.name == "关于") {
						if err := os.MkdirAll(dir, 0700); err != nil {
							t.Fatal(err)
						}
						file, err := os.Create(filepath.Join(dir, mode+"-"+tab.name+".png"))
						if err != nil {
							t.Fatal(err)
						}
						err = png.Encode(file, tt.Image())
						closeErr := file.Close()
						if err != nil {
							t.Fatal(err)
						}
						if closeErr != nil {
							t.Fatal(closeErr)
						}
					}
				}
			})
		}
	}
}

func TestAboutCheckUpdates(t *testing.T) {
	previous := checkForUpdates
	defer func() { checkForUpdates = previous }()
	called := false
	checkForUpdates = func() { called = true }
	a := &application{showSettings: true, settingsAboutTab: true}
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("check for updates was not invoked")
	}
}

func TestPlayerPathDraftAndImmediateEngineSelection(t *testing.T) {
	a := &application{showSettings: true, settingsPlayerTab: true, path: filepath.Join(t.TempDir(), "state.json")}
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("mpv 路径"); err != nil {
		t.Fatal(err)
	}
	tt.Type("C:/tools/mpv.exe")
	if a.playerPathDraft != "C:/tools/mpv.exe" || a.state.Player != "" {
		t.Fatal("path editing modified live preferences")
	}
	if err := tt.Click("选择 内置 WebView"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(a.path)
	if err != nil || loaded.PlaybackEngine != "webview" || loaded.Player != "" {
		t.Fatalf("engine selection saved draft path: %+v %v", loaded, err)
	}
	if a.webPlayer != nil || a.webView != nil {
		t.Fatal("selection eagerly initialized WebView")
	}
	if err := tt.Click("选择 mpv 播放器（默认）"); err != nil {
		t.Fatal(err)
	}
	for _, tab := range []string{"关于", "备份", "播放器"} {
		if err := tt.Click(tab); err != nil {
			t.Fatal(err)
		}
	}
	if a.playerPathDraft != "C:/tools/mpv.exe" || a.state.Player != "" {
		t.Fatal("tab/engine switch lost draft separation")
	}
	if err := tt.Click("保存设置"); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load(a.path)
	if err != nil || loaded.Player != "C:/tools/mpv.exe" || !strings.Contains(a.status, "已保存") {
		t.Fatalf("path save not committed: %+v %s %v", loaded, a.status, err)
	}
	a.installImportedConfig(store.State{Player: "imported-mpv"}, false)
	tt.Frame()
	if a.playerPathDraft != "imported-mpv" {
		t.Fatal("import retained stale path draft")
	}
}

func TestPathSaveFeedbackWaitsForBackgroundResult(t *testing.T) {
	a, _, queue, _ := playbackFixture(t)
	a.startBackground()
	t.Cleanup(a.cleanup)
	a.playerPathDraft = "new-mpv"
	a.playerPathDraftLoaded = true
	a.savePlayerPath()
	if !strings.Contains(a.status, "已提交保存") || strings.Contains(a.status, "已保存") {
		t.Fatal("pending save reported success")
	}
	if err := a.background.Close(); err != nil {
		t.Fatal(err)
	}
	pumpPlayback(t, queue, func() bool { return strings.Contains(a.status, "已保存") })
}

func TestSettingsStatusSeverityAndWrappedDetail(t *testing.T) {
	for _, theme := range []*ui.Theme{lightAppTheme, appTheme} {
		for _, tc := range []struct{ message, label string }{
			{"保存设置失败；此前已保存旧配置", "错误"}, {"配置未恢复，保存已禁用", "注意"},
			{"路径尚未保存", "注意"}, {"播放器路径已提交保存，下次播放生效", "处理中"},
			{"正在下载", "处理中"}, {"配置已导入", "成功"}, {"已导出 3 个订阅", "成功"}, {"准备就绪", "提示"},
		} {
			label, _ := statusStyle(tc.message, theme)
			if label != tc.label {
				t.Fatalf("%q: got %s want %s", tc.message, label, tc.label)
			}
		}
	}
	a := &application{showSettings: true, settingsPlayerTab: true}
	a.setSettingsError("player-path", "路径选择失败，请检查路径")
	tt := ui.NewTester(a.view, 1000, 620)
	contained(t, "播放器路径错误", requiredRect(t, tt, "播放器配置"), requiredRect(t, tt, "设置错误 player-path"))
	a.configError = errors.New("test corruption")
	a.status = "配置已导入"
	tt.Frame()
	if _, found := tt.Find("全局配置错误"); !found {
		t.Fatal("unresolved config failure hidden by another status")
	}
}

func TestCompatibilityDisclosureAndAboutDefaultVersion(t *testing.T) {
	a := &application{showSettings: true, settingsPlayerTab: true}
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("兼容性说明"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(250 * time.Millisecond)
	if !a.playerCompatibilityOpen || !tt.HasText("mpv：仅支持 mpv 可执行程序，不支持 VLC 等其他播放器；Windows 画面内嵌，macOS/Linux 使用独立窗口，应用内暂停/音量暂不可用。") {
		t.Fatal("compatibility disclosure did not open")
	}
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("版本：dev（开发构建）") {
		t.Fatal("missing development build version")
	}
}
