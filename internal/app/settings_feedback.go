package app

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/l10n"
)

func settingsDanger(t *ui.Theme) ui.Color {
	if !t.Dark {
		return ui.Hex("#b91c1c")
	}
	return t.Danger
}

func settingsWarning(t *ui.Theme) ui.Color {
	if !t.Dark {
		return ui.Hex("#8a5300")
	} // Keep small warning text legible on light surfaces.
	return t.Warning
}

// Existing services publish human-readable status strings. Keep their original
// detail and classify conservatively: failure always wins over success words.
func statusStyle(status string, t *ui.Theme) (string, ui.Color) {
	lowerStatus := strings.ToLower(status)
	groups := []struct {
		label string
		color ui.Color
		words []string
	}{
		{"错误", settingsDanger(t), []string{"失败", "未找到", "无法", "错误", "无效", "损坏", "failed", "not found", "unable", "error", "invalid", "corrupt"}},
		{"注意", settingsWarning(t), []string{"未恢复", "已禁用", "禁止", "超时", "所有线路", "不支持", "不可用", "尚未保存", "没有剩余", "not recovered", "disabled", "timed out", "all routes", "not supported", "unavailable", "not saved yet", "no compatible"}},
		{"处理中", t.Accent, []string{"正在", "已提交保存", "连接中", "queued for saving", "connecting", "refreshing", "preparing", "loading", "cleaning"}},
		{"成功", paletteFor(t).Success, []string{"已保存", "已导出", "配置已导入", "已切换", "更新完成", "更新成功", "saved", "exported", "settings imported", "changed", "refresh complete", "refreshed successfully"}},
	}
	for _, group := range groups {
		for _, word := range group.words {
			if strings.Contains(lowerStatus, word) {
				return tr(group.label), group.color
			}
		}
	}
	return tr("提示"), t.TextMuted
}

func (a *application) setSettingsError(area, message string) {
	if a.settingsErrors == nil {
		a.settingsErrors = make(map[string]string)
	}
	if message == "" {
		delete(a.settingsErrors, area)
	} else {
		a.settingsErrors[area] = message
	}
}

func (a *application) settingsError(c *ui.Context, area string) {
	if message := a.settingsErrors[area]; message != "" {
		ui.Text(c, message).Label(tr("设置错误 ") + area).FontSize(12).TextColor(settingsDanger(c.Theme()))
	}
}

func (a *application) settingsStatus(c *ui.Context) {
	message := a.saveError
	if a.configError != nil {
		message = tr("配置未恢复，保存已禁用：") + a.configError.Error()
	}
	if message != "" {
		ui.Text(c, message).Label(tr("全局配置错误")).FontSize(12).TextColor(settingsDanger(c.Theme()))
	}
}

func (a *application) showSettingsToast(message string) {
	if a.settingsToastTimer != nil {
		a.settingsToastTimer.Stop()
	}
	a.settingsToast = message
	a.settingsToastGeneration++
	generation := a.settingsToastGeneration
	if a.window == nil && a.dispatch == nil {
		return
	}
	a.settingsToastTimer = time.AfterFunc(5*time.Second, func() {
		a.update(func() { a.dismissSettingsToast(generation) })
	})
}

func (a *application) dismissSettingsToast(generation uint64) {
	if a.settingsToastGeneration == generation {
		a.settingsToast = ""
	}
}

func (a *application) settingsToastView(c *ui.Context) {
	if a.settingsToast == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).Label(tr("设置浮动提示")).PassThrough().Absolute().Right(24).Bottom(24).MaxWidth(480).Padding(14, 18).Gap(10).Radius(10).Background(t.Surface).Border(1, paletteFor(t).Success.Alpha(.6)).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, 0.18)).Children(func() {
		ui.Text(c, tr("成功")).PassThrough().FontSize(12).Bold().TextColor(paletteFor(t).Success)
		ui.Text(c, a.settingsToast).PassThrough().FontSize(12).TextColor(t.Text)
	})
}

func (a *application) finishSettingsSave() {
	// Do not replace a newer playback/import/error message with an old save result.
	for _, tag := range []string{"zh-Hans", "en"} {
		pending := l10n.In(tag, "已提交保存")
		if strings.Contains(a.status, pending) {
			a.status = strings.Replace(a.status, pending, l10n.In(tag, "已保存"), 1)
			break
		}
	}
	if a.pendingSettingsSuccess != "" {
		a.showSettingsToast(a.pendingSettingsSuccess)
		a.pendingSettingsSuccess = ""
	}
}
