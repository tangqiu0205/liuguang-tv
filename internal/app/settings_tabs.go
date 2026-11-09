package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"strings"
)

func (a *application) settingsTabs(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Label(tr("设置标签栏")).Width(180).Shrink(0).MinHeight(0).DragWindow().Padding(20, 12).Gap(6).Background(panelBackground(t)).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Height(54).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Image(c, appIcon).Label(tr("设置应用图标")).Size(32, 32).Shrink(0)
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, tr("流光 TV")).FontSize(15).Bold()
				ui.Text(c, tr("直播 · 频道 · 节目单")).FontSize(10).TextColor(t.TextMuted)
			})
		})
		ui.Text(c, tr("设置")).Padding(12, 8, 6, 8).FontSize(12).Bold().TextColor(t.TextMuted)
		for _, item := range []struct {
			name string
			icon *ui.SVG
		}{
			{"订阅源", sourceLayersIcon}, {"播放器", playIcon}, {"外观", windowExpandIcon}, {"备份", backupIcon}, {"关于", infoIcon},
		} {
			tab := item.name
			b := ui.ButtonBase(c.Key(tab)).Label(tr(tab)).FillWidth().Height(42).Padding(0, 12).Radius(8).Justify(ui.Start).Gap(12)
			selected := (!a.settingsPlayerTab && !a.settingsAppearanceTab && !a.settingsBackupTab && !a.settingsAboutTab && tab == "订阅源") || (a.settingsPlayerTab && tab == "播放器") || (a.settingsAppearanceTab && tab == "外观") || (a.settingsBackupTab && tab == "备份") || (a.settingsAboutTab && tab == "关于")
			bg, color := ui.Transparent, t.Text
			if selected {
				bg, color = selectedBackground(t), t.Accent
			} else if b.Hovered() {
				bg = t.SurfaceHover
			}
			b.Background(bg).Children(func() {
				ui.Icon(c, item.icon).Size(18, 18).TextColor(color)
				ui.Text(c, tr(tab)).FontSize(13).Bold().TextColor(color)
			})
			if b.Clicked() {
				a.settingsEPGTab = false
				a.settingsPlayerTab = tab == "播放器"
				a.settingsAppearanceTab = tab == "外观"
				a.settingsBackupTab = tab == "备份"
				a.settingsAboutTab = tab == "关于"
			}
		}
	})
}

func (a *application) subscriptionTabs(c *ui.Context) {
	t := c.Theme()
	count := len(a.state.Subscriptions)
	if a.settingsEPGTab {
		count = len(a.state.ManualEPGSources())
	}
	ui.Column(c).Label(tr("订阅源区域标题")).Padding(8, 24, 0, 24).Shrink(0).Gap(10).Children(func() {
		ui.Row(c).Height(40).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Icon(c, sourceLayersIcon).Size(23, 23).TextColor(t.Accent)
			ui.Text(c, tr("订阅源")).FontSize(20).Bold()
			ui.Textf(c, "%d", count).Label(tr("订阅数量")).FontSize(14).Bold().TextColor(t.Accent).Background(selectedBackground(t)).Radius(8).Padding(3, 9)
			ui.Spacer(c)
			if settingsAction(c, tr("顶部添加自定义源"), tr("添加订阅源"), addIcon, true, false).Clicked() {
				a.openSubscriptionEditor()
			}
			if !a.settingsEPGTab {
				if settingsAction(c, tr("刷新全部"), tr("刷新全部"), refreshIcon, false, len(a.state.Subscriptions) == 0 || len(a.refreshing) > 0).Clicked() {
					a.refreshAll()
				}
			}
		})
		ui.Row(c).Label(tr("订阅源子标签栏")).Height(34).Gap(8).Children(func() {
			for _, tab := range []string{"订阅管理", "EPG 节目单"} {
				selected := a.settingsEPGTab == (tab == "EPG 节目单")
				b := ui.ButtonBase(c.Key(tab)).Label(tr(tab)).MinWidth(140).Height(32).Radius(7).Gap(7)
				bg, color := ui.Transparent, t.TextMuted
				if selected {
					bg, color = selectedBackground(t), t.Accent
				} else if b.Hovered() {
					bg = t.SurfaceHover
				}
				b.Background(bg).BorderWidth(0, 0, 2, 0).BorderColor(selectionBorder(t, selected)).Children(func() {
					icon := sourceLayersIcon
					if tab == "EPG 节目单" {
						icon = calendarIcon
					}
					ui.Icon(c, icon).Size(16, 16).TextColor(color)
					ui.Text(c, tr(tab)).FontSize(12).TextColor(color)
				})
				if b.Clicked() {
					a.settingsEPGTab = tab == "EPG 节目单"
				}
			}
		})
		if a.settingsEPGTab {
			ui.Text(c, tr("优先使用订阅自带节目单；无可用匹配时，按下方来源顺序依次匹配，可用左右箭头调整优先级。")).FontSize(11).TextColor(t.TextMuted).FillWidth()
		}
	})
}

func (a *application) openSubscriptionEditor() {
	a.showSubEditor = true
	a.subEPG = a.settingsEPGTab
	a.editingEPGID = ""
	a.editingID = ""
	a.subName = ""
	a.source = ""
	a.deletingID = ""
	a.editorScroll.ScrollTo(0, 0, ui.Animation{})
}

func (a *application) pickPlayerPath() {
	a.transferBusy = true
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: tr("选择 mpv 可执行程序")})
		a.update(func() {
			a.transferBusy = false
			if err != nil {
				a.status = tr("选择 mpv 程序失败")
				a.setSettingsError("player-path", a.status)
				return
			}
			if len(paths) > 0 {
				a.setSettingsError("player-path", "")
				a.playerPathDraft = paths[0]
				a.playerPathDraftLoaded = true
				a.status = tr("已选择 mpv 路径，尚未保存")
			}
		})
	}()
}

func (a *application) savePlayerPath() {
	a.setSettingsError("player-path", "")
	a.state.Player = strings.TrimSpace(a.playerPathDraft)
	a.playerPathDraft = a.state.Player
	a.status = tr("播放器路径已提交保存，下次播放生效")
	a.pendingSettingsSuccess = tr("播放器路径已保存，下次播放生效")
	a.save()
}

const mpvDownloadURL = "https://mpv.io/installation/"

var openPlayerExternal = mygo.Shell.OpenExternal

func (a *application) openMPVDownload() {
	if err := openPlayerExternal(mpvDownloadURL); err != nil {
		a.setSettingsError("player-download", tr("无法打开 mpv 下载页面，请手动访问：")+mpvDownloadURL+"\n"+err.Error())
		return
	}
	a.setSettingsError("player-download", "")
}

func (a *application) playerSettingsPanel(c *ui.Context) {
	if !a.playerPathDraftLoaded {
		a.playerPathDraft = a.state.Player
		a.playerPathDraftLoaded = true
	}
	t := c.Theme()
	ui.Scroll(c.Key("player-page")).Label(tr("播放器页面")).TrackScroll(&a.playerSettingsScroll).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(24).Gap(16).Children(func() {
			ui.Column(c).Label(tr("播放器配置")).Padding(18).Radius(12).Background(panelBackground(t)).Border(1, t.Border).Gap(12).Children(func() {
				ui.Text(c, tr("播放器设置")).FontSize(16).Bold()
				ui.Row(c).Label(tr("播放引擎")).Gap(12).Children(func() {
					for _, option := range []struct{ engine, title, detail string }{
						{"mpv", tr("mpv 播放器（默认）"), tr("兼容更多协议与编码，需要安装 mpv。")},
						{"webview", tr("内置 WebView"), tr("无需 mpv，使用系统 WebView 播放。")},
					} {
						selected := a.webPlayback() == (option.engine == "webview")
						b := ui.ButtonBase(c.Key(option.engine)).Label(tr("选择 ") + option.title).Grow(1).MinWidth(0).Height(88).Padding(14).Radius(10)
						bg, border := t.Surface, t.Border
						if selected {
							bg, border = selectedBackground(t), t.Accent
						} else if b.Hovered() {
							bg = t.SurfaceHover
						}
						b.Background(bg).Border(1, border).Children(func() {
							ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
								ui.Row(c).Gap(8).Children(func() {
									ui.Text(c, option.title).FontSize(13).Bold().Grow(1).MinWidth(0)
									if selected {
										ui.Text(c, tr("已选")).FontSize(10).Bold().TextColor(t.Accent).Shrink(0)
									}
								})
								ui.Text(c, option.detail).FontSize(11).TextColor(t.TextMuted)
							})
						})
						if b.Clicked() {
							a.setPlaybackEngine(option.engine)
						}
					}
				})
				ui.Text(c, tr("引擎选择立即生效并自动保存；切换会停止播放，需重新选择频道。")).FontSize(11).TextColor(t.TextMuted)
				if !a.webPlayback() {
					ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
						ui.Text(c, tr("mpv 程序路径")).FontSize(12).Bold().Grow(1)
						if ui.Button(c, tr("下载 mpv")).Height(28).Tooltip(mpvDownloadURL).Clicked() {
							a.openMPVDownload()
						}
					})
					ui.Row(c).Gap(10).Children(func() {
						ui.TextInput(c.Key("player"), &a.playerPathDraft).Label(tr("mpv 路径")).Placeholder(tr("留空自动查找 mpv")).Grow(1).MinWidth(0)
						if ui.Button(c, tr("浏览…")).Label(tr("选择 mpv 程序")).Clicked() {
							a.pickPlayerPath()
						}
						if ui.Button(c, tr("保存设置")).Disabled(a.configError != nil).Clicked() {
							a.savePlayerPath()
						}
					})
					hint := tr("路径留空自动查找；修改后点击保存，下次播放生效，不重启当前播放。")
					color := t.TextMuted
					if a.playerPathDraft != a.state.Player {
						hint = tr("路径尚未保存，不影响当前设置；点击“保存设置”后生效。")
						color = settingsWarning(t)
					}
					ui.Text(c, hint).FontSize(11).TextColor(color)
					a.settingsError(c, "player-path")
					a.settingsError(c, "player-download")
				}
			})
			ui.Collapsible(c, tr("兼容性说明"), &a.playerCompatibilityOpen, func() {
				ui.Text(c, tr("mpv：仅支持 mpv 可执行程序，不支持 VLC 等其他播放器；Windows 画面内嵌，macOS/Linux 使用独立窗口，应用内暂停/音量暂不可用。")).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, tr("WebView：支持 HLS、HTTP-FLV、HTTP-TS 和系统支持的直链媒体，编解码能力取决于系统；Windows 需要 WebView2 Runtime，Linux 需要 WebKitGTK。不支持时可手动改用 mpv，不会自动切换引擎。")).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, tr("切回 mpv 时销毁 WebView、关闭本机服务，下次播放再按需创建；系统浏览器共享进程和缓存不保证立即全部释放。")).FontSize(11).TextColor(t.TextMuted)
			}).FillWidth()
		})
	})
}
