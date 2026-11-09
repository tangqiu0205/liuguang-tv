package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/statework"
)

func TestSettingsSaveFeedbackRejectsStaleAndFailedResults(t *testing.T) {
	a := &application{stateSequence: 2, status: "播放器路径已提交保存，下次播放生效"}
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 1}}, channelIndex{})
	if !strings.Contains(a.status, "已提交保存") {
		t.Fatal("stale save reported success")
	}
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 2}, Err: errors.New("disk full")}, channelIndex{})
	if !strings.Contains(a.status, "失败") || strings.Contains(a.status, "已保存") {
		t.Fatal("failed save reported success")
	}
	a.status = "正在播放：Demo"
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 2}}, channelIndex{})
	if a.status != "正在播放：Demo" {
		t.Fatal("save result overwrote a newer unrelated status")
	}
}

func TestSettingsSaveToastWaitsForLatestSuccessfulWrite(t *testing.T) {
	a := &application{stateSequence: 2, pendingSettingsSuccess: "主题设置已保存"}
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 1}}, channelIndex{})
	if a.settingsToast != "" {
		t.Fatal("stale write showed success")
	}
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 2}, Err: errors.New("disk full")}, channelIndex{})
	if a.settingsToast != "" || a.saveError == "" {
		t.Fatal("failed write did not retain global error")
	}
	a.stateSequence++
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 3}}, channelIndex{})
	if a.settingsToast != "主题设置已保存" || a.saveError != "" {
		t.Fatal("successful retry did not clear error and report success")
	}
}

func TestSettingsToastAndErrorsAreIndependent(t *testing.T) {
	a := &application{status: "订阅更新完成，正在整理频道…", refreshResult: subscriptionRefreshResult{completed: 1, name: "测试源"}}
	a.setSettingsError("player-path", "路径错误")
	a.setSettingsError("import", "导入失败")
	a.finishRefreshStatus()
	generation := a.settingsToastGeneration
	if !strings.Contains(a.settingsToast, "更新完成") {
		t.Fatalf("missing toast: %q", a.settingsToast)
	}
	a.showSettingsToast("设置已保存")
	a.dismissSettingsToast(generation)
	if a.settingsToast != "设置已保存" {
		t.Fatal("old dismissal cleared a newer toast")
	}
	a.dismissSettingsToast(a.settingsToastGeneration)
	if a.settingsToast != "" || a.settingsErrors["player-path"] == "" || a.settingsErrors["import"] == "" {
		t.Fatal("dismissal did not preserve independent errors")
	}
	a.setSettingsError("player-path", "")
	if a.settingsErrors["import"] == "" {
		t.Fatal("clearing one error cleared another")
	}
}

func TestSettingsToastFloatsAndErrorsStayInTheirAreas(t *testing.T) {
	a := &application{showSettings: true, settingsBackupTab: true}
	a.setSettingsError("export", "导出失败")
	a.setSettingsError("import", "导入失败")
	a.showSettingsToast("设置已保存")
	tt := ui.NewTester(a.view, 1000, 620)
	contained(t, "导出错误", requiredRect(t, tt, "导出备份区域"), requiredRect(t, tt, "设置错误 export"))
	contained(t, "导入错误", requiredRect(t, tt, "恢复配置区域"), requiredRect(t, tt, "设置错误 import"))
	toast := requiredRect(t, tt, "设置浮动提示")
	if toast.Y < 400 {
		t.Fatalf("toast is not floating near the bottom: %+v", toast)
	}
	a.dismissSettingsToast(a.settingsToastGeneration)
	tt.Frame()
	if _, found := tt.Find("设置浮动提示"); found {
		t.Fatal("dismissed toast is still visible")
	}
	if _, found := tt.Find("设置错误 export"); !found {
		t.Fatal("toast dismissal removed local error")
	}
}

func TestSettingsFeedbackTextContrast(t *testing.T) {
	for _, theme := range []*ui.Theme{lightAppTheme, appTheme} {
		for _, message := range []string{"失败", "尚未保存", "正在保存", "已保存", "准备就绪"} {
			_, fg := statusStyle(message, theme)
			blend := func(fg, bg uint8) uint8 { return uint8(.07*float64(fg) + .93*float64(bg)) }
			bg := ui.Color{R: blend(fg.R, theme.Background.R), G: blend(fg.G, theme.Background.G), B: blend(fg.B, theme.Background.B), A: 255}
			light, dark := luminance(fg), luminance(bg)
			if light < dark {
				light, dark = dark, light
			}
			if ratio := (light + .05) / (dark + .05); ratio < 4.5 {
				t.Errorf("dark=%v message=%s contrast=%.2f", theme.Dark, message, ratio)
			}
		}
	}
}
