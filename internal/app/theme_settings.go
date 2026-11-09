package app

import "github.com/egoist/mygo/ui"

func (a *application) appearanceSettingsPanel(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c.Key("appearance-page")).Label(tr("外观页面")).TrackScroll(&a.appearanceScroll).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(24).Gap(20).Children(func() {
			a.languageSettings(c)
			ui.Column(c).Label(tr("主题设置")).Padding(20).Radius(12).Background(panelBackground(t)).Border(1, t.Border).Gap(14).Children(func() {
				ui.Text(c, tr("主题模式")).FontSize(16).Bold()
				ui.Text(c, tr("默认跟随系统；也可固定使用日间或夜间。选择后立即生效并自动保存，不影响视频播放。")).FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Label(tr("主题选项")).Gap(10).Children(func() {
					for _, option := range []struct{ mode, label string }{{"system", tr("跟随系统")}, {"light", tr("日间")}, {"dark", tr("夜间")}} {
						selected := normalizedThemeMode(a.state.ThemeMode) == option.mode
						bg, color, border := t.Surface, t.Text, t.Border
						button := ui.ButtonBase(c.Key(option.mode)).Label(tr("主题选项 ") + option.label).Height(40).Grow(1).MinWidth(0).Radius(9)
						if selected {
							bg, color, border = selectedBackground(t), t.Accent, selectionBorder(t, true)
						} else if button.Hovered() {
							bg = t.SurfaceHover
						}
						button.Background(bg).Border(1, border).Children(func() { ui.Text(c, option.label).FontSize(13).Bold().TextColor(color) })
						if button.Clicked() {
							a.setThemeMode(option.mode)
							c.Invalidate()
						}
					}
				})
				effective := tr("日间")
				if t.Dark {
					effective = tr("夜间")
				}
				ui.Text(c, tr("当前显示：")+effective).FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}
