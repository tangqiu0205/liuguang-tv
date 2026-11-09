package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

const projectURL = "https://github.com/tangqiu0205/liuguang-tv"

var checkForUpdates = updater.CheckForUpdates

func (a *application) openProjectURL(target string) {
	if err := mygo.Shell.OpenExternal(target); err != nil {
		a.status = tr("打开项目链接失败，请在浏览器访问：") + target
		a.setSettingsError("about", a.status)
	} else {
		a.setSettingsError("about", "")
	}
}

func (a *application) aboutSettingsPanel(c *ui.Context) {
	t := c.Theme()
	version := a.versionLabel()
	if version == "dev" {
		version += tr("（开发构建）")
	}
	ui.Scroll(c.Key("about-page")).Label(tr("关于页面")).TrackScroll(&a.aboutScroll).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(24).Gap(16).Children(func() {
			ui.Column(c).Label(tr("应用信息")).Padding(20).Gap(12).Radius(12).Background(panelBackground(t)).Border(1, t.Border).Children(func() {
				ui.Row(c).Gap(14).Children(func() {
					ui.Column(c).Size(52, 52).Radius(14).Center().Background(selectedBackground(t)).Children(func() { ui.Icon(c, tvIcon).Size(28, 28).TextColor(t.Accent) })
					ui.Column(c).Grow(1).Gap(5).Children(func() {
						ui.Text(c, tr("流光 TV")).FontSize(21).Bold()
						ui.Column(c).Label(tr("应用版本")).Children(func() { ui.Text(c, tr("版本：")+version).FontSize(13).Bold().TextColor(t.Accent) })
						ui.Text(c, tr("Liuguang TV · 原生界面的 IPTV 桌面客户端")).FontSize(12).TextColor(t.TextMuted)
					})
				})
				ui.Row(c).Gap(10).Children(func() {
					if ui.Button(c, tr("检查更新")).Clicked() {
						checkForUpdates()
					}
					if ui.Button(c, tr("项目主页")).Clicked() {
						a.openProjectURL(projectURL)
					}
					if ui.Button(c, tr("问题反馈")).Clicked() {
						a.openProjectURL(projectURL + "/issues")
					}
				})
				a.settingsError(c, "about")
			})
			ui.Text(c, tr("内容与隐私：应用不提供节目源，请仅使用有授权的来源。订阅、收藏和设置保存在本机；订阅/EPG 刷新及播放会连接所配置的来源。备份可能包含地址凭据，不应公开分享。")).Label(tr("内容与隐私说明")).FontSize(11).TextColor(t.TextMuted)
		})
	})
}
