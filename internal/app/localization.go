package app

import (
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/l10n"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func tr(source string) string { return l10n.T(source) }

// Group values are data, not translated identifiers. Only the built-in
// sentinel has a localized presentation; actual group names stay untouched.
func groupTitle(group string) string {
	if group == "全部分组" {
		return tr("全部分组")
	}
	return group
}

func (a *application) languageSettings(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Label(tr("语言")).Padding(20).Radius(12).Background(panelBackground(t)).Border(1, t.Border).Gap(14).Children(func() {
		ui.Text(c, tr("界面语言")).FontSize(16).Bold()
		ui.Text(c, tr("默认跟随系统，也可选择简体中文或英文。立即生效并自动保存，不影响播放；频道和节目名称保持原文。")).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Label(tr("语言选项")).Gap(10).Children(func() {
			for _, option := range []struct{ tag, name string }{{"", tr("跟随系统")}, {"zh-Hans", "简体中文"}, {"en", "English"}} {
				selected := a.languagePreference == option.tag && a.settingsErrors["language"] == ""
				button := ui.ButtonBase(c.Key(option.tag)).Label(tr("语言选项 ") + option.name).Height(40).Grow(1).MinWidth(0).Radius(9)
				bg, color, border := t.Surface, t.Text, t.Border
				if selected {
					bg, color, border = selectedBackground(t), t.Accent, selectionBorder(t, true)
				} else if button.Hovered() {
					bg = t.SurfaceHover
				}
				button.Background(bg).Border(1, border).Children(func() { ui.Text(c, option.name).FontSize(13).Bold().TextColor(color) })
				if button.Clicked() {
					a.setLanguage(option.tag)
					c.Invalidate()
				}
			}
		})
		current := "简体中文"
		if i18n.Locale() == "en" {
			current = "English"
		}
		ui.Text(c, tr("当前语言：")+current).FontSize(11).TextColor(t.TextMuted)
		a.settingsError(c, "language")
	})
}

var changeAppLocale = mygo.App.SetLocale
var loadAppLanguagePreference = readLanguagePreference

func (a *application) setLanguage(tag string) {
	changeAppLocale(tag)
	a.refreshLanguagePreference()
	if a.settingsErrors["language"] == "" && a.languagePreference != tag {
		a.setSettingsError("language", tr("语言选择未保存；请检查数据目录权限或 MYGO_LOCALE 环境变量。"))
	}
}

func (a *application) refreshLanguagePreference() {
	tag, err := loadAppLanguagePreference(filepath.Dir(a.path))
	if err != nil {
		a.setSettingsError("language", tr("读取语言偏好失败：")+err.Error())
		return
	}
	a.languagePreference = tag
	a.setSettingsError("language", "")
}

func (a *application) localeChanged(string) {
	a.refreshLanguagePreference()
	if a.window == nil || a.closed {
		return
	}
	a.window.SetTitle(tr("流光 TV"))
	// A route's label is presentation only; its URL remains the identity.
	routes := a.playbackRoutes()
	for i, line := range playlist.Lines(routes) {
		if line.URL == a.nowURL {
			a.selectedLine = lineLabels(routes)[i]
			break
		}
	}
	a.window.Invalidate()
}
