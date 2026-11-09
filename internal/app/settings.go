package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func (a *application) statusHint(c *ui.Context) ui.Element {
	status := a.status
	if status == "" {
		status = tr("准备就绪")
	}
	_, color := statusStyle(status, c.Theme())
	return ui.Text(c, status).Label(tr("状态提示")).Tooltip(status).FontSize(11).TextColor(color).SingleLine()
}

func (a *application) settingsPanel(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	editorOpen := a.showSubEditor && a.subEPG == a.settingsEPGTab
	editorWidth := min(float32(400), width*.4)
	listWidth := width - 212
	if editorOpen {
		listWidth -= editorWidth
	}
	ui.Row(c).Label(tr("设置页面")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		a.settingsTabs(c)
		ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
			ui.Column(c).Padding(max(16, c.TitleBar().Height), 24, 16, 24).Gap(6).Shrink(0).Children(func() {
				ui.Row(c).Label(tr("设置顶栏")).MinHeight(56).DragWindow().AlignItems(ui.Center).Gap(12).Children(func() {
					ui.Column(c).Label(tr("设置标题图标")).Size(44, 44).Radius(14).Background(selectedBackground(t)).Center().Children(func() {
						ui.Icon(c, gearIcon).Size(27, 27).TextColor(t.Accent)
					})
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, tr("设置")).Label(tr("设置标题")).FontSize(26).Bold()
						ui.Text(c, tr("管理订阅源、播放器、外观与备份；订阅源和 EPG 每 24 小时自动更新一次，也可手动刷新。")).Label(tr("设置说明")).FontSize(12).TextColor(t.TextMuted)
					})
					if settingsAction(c, tr("返回播放"), tr("返回播放"), playIcon, false, false).Clicked() {
						a.showSettings = false
						a.settle = 3
					}
				})
				a.settingsStatus(c)
			})
			ui.Column(c).Padding(0, 16, 16, 16).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Label(tr("设置内容区")).Grow(1).MinHeight(0).Radius(14).Background(panelBackground(t)).Border(1, t.Border).Clip().Children(func() {
					if a.settingsAppearanceTab {
						a.appearanceSettingsPanel(c)
					} else if a.settingsPlayerTab {
						a.playerSettingsPanel(c)
					} else if a.settingsBackupTab {
						a.backupSettingsPanel(c)
					} else if a.settingsAboutTab {
						a.aboutSettingsPanel(c)
					} else {
						a.subscriptionTabs(c)
						ui.Row(c.Key("subscriptions-page")).Label(tr("设置分栏")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
							if a.settingsEPGTab {
								ui.Column(c).Label(tr("EPG 页面")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
									if len(a.state.ManualEPGSources()) == 0 {
										a.epgSettings(c, listWidth)
									} else {
										ui.Scroll(c.Key("epg-page")).TrackScroll(&a.epgSettingsScroll).Grow(1).MinHeight(0).Children(func() {
											ui.Column(c).Padding(24).Children(func() { a.epgSettings(c, listWidth) })
										})
									}
								})
							} else {
								ui.Scroll(c.Key("subscription-list-scroll")).Label(tr("设置列表")).TrackScroll(&a.settingsScroll).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
									ui.Column(c).Padding(24).Gap(20).Children(func() { a.subscriptionList(c, listWidth) })
								})
							}
							if editorOpen {
								ui.Column(c).Label(tr("订阅编辑栏")).Width(editorWidth).Shrink(0).MinHeight(0).Background(panelBackground(t)).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Clip().Children(func() {
									a.subscriptionEditor(c)
								})
							}
						})
					}
				})
			})
		})
	})
}

func (a *application) subscriptionEditor(c *ui.Context) {
	t := c.Theme()
	title, save, cancel := tr("添加自定义源"), tr("添加并更新"), tr("取消添加")
	if a.editingID != "" || a.editingEPGID != "" {
		title, save, cancel = tr("编辑订阅源"), tr("保存并更新"), tr("取消编辑")
	}
	description := tr("添加网络频道列表，或从本地文件导入。")
	help := tr("支持 M3U / M3U8 / TXT 频道列表，不是单个频道的 HLS 播放地址。保存后加载频道，每个订阅独立缓存。")
	if a.subEPG {
		description = tr("添加 XMLTV 节目单，作为频道的备用 EPG 来源。")
		help = tr("支持 XMLTV / XMLTV.gz 网络地址或本地文件。优先使用频道订阅自带节目单，无匹配时使用此来源。")
	}
	ui.Column(c).Label(tr("M3U 订阅设置")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Label(tr("订阅编辑标题")).Padding(20).Gap(10).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Size(36, 36).Radius(10).Background(selectedBackground(t)).Center().Children(func() {
					ui.Icon(c, sourceLayersIcon).Size(21, 21).TextColor(t.Accent)
				})
				ui.Text(c, title).FontSize(17).Bold().Grow(1).MinWidth(0)
				if iconButton(c, tr("关闭订阅编辑"), closeIcon, false, false).Clicked() {
					a.cancelSubscriptionEdit()
				}
			})
			ui.Text(c, description).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Scroll(c.Key("subscription-editor-fields")).Label(tr("编辑栏滚动区")).TrackScroll(&a.editorScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(20).Gap(16).Children(func() {
				ui.Column(c).Label(tr("订阅基本信息")).Padding(16).Gap(18).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
					ui.Text(c, tr("基本信息")).FontSize(13).Bold()
					ui.Column(c).Gap(7).Children(func() {
						ui.Text(c, tr("订阅类型")).FontSize(12).Bold()
						ui.Row(c).Gap(6).Children(func() {
							for _, option := range []struct {
								label string
								epg   bool
							}{{tr("频道订阅"), false}, {tr("EPG 节目单"), true}} {
								b := ui.ButtonBase(c.Key(option.epg)).Label(tr("类型：") + option.label).Grow(1).MinWidth(0).Height(32).Radius(7).Disabled(a.busy || a.editingID != "" || a.editingEPGID != "")
								selected := a.subEPG == option.epg
								bg, color := t.Background, t.TextMuted
								if selected {
									bg, color = selectedBackground(t), t.Accent
								}
								b.Background(bg).Border(1, selectionBorder(t, selected)).Children(func() { ui.Text(c, option.label).FontSize(11).TextColor(color) })
								if b.Clicked() {
									a.subEPG, a.settingsEPGTab = option.epg, option.epg
								}
							}
						})
					})
					ui.Column(c).Gap(7).Children(func() {
						ui.Text(c, tr("订阅名称（可选）")).FontSize(12).Bold()
						ui.TextInput(c.Key("subscription-name"), &a.subName).Label(tr("订阅名称")).Placeholder(tr("例如：家庭直播源")).FillWidth()
						ui.Text(c, tr("留空时根据地址自动命名。")).FontSize(10).TextColor(t.TextMuted)
					})
					ui.Column(c).Gap(7).Children(func() {
						ui.Text(c, tr("订阅地址或文件路径")).FontSize(12).Bold()
						ui.TextInput(c.Key("source"), &a.source).Label(tr("订阅地址或文件路径")).Placeholder(tr("HTTP(S) 地址或本地文件路径")).FillWidth()
						if settingsAction(c, tr("选择本地文件…"), tr("选择本地文件…"), sourceLayersIcon, false, a.busy).FillWidth().Clicked() {
							a.pickSubscriptionFile()
						}
					})
				})
				ui.Row(c).Padding(12).Gap(8).Radius(10).Background(selectedBackground(t)).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, infoIcon).Size(16, 16).TextColor(t.Accent).Shrink(0)
					ui.Text(c, help).FontSize(11).TextColor(t.TextMuted).Grow(1).MinWidth(0)
				})
			})
		})
		ui.Column(c).Label(tr("订阅编辑操作区")).Padding(14, 20).Gap(10).Shrink(0).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			a.settingsError(c, "subscription-editor")
			ui.Row(c).Gap(10).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, cancel).Height(34).Clicked() {
					a.cancelSubscriptionEdit()
				}
				if settingsAction(c, save, save, addIcon, true, a.busy || strings.TrimSpace(a.source) == "").Clicked() {
					a.saveSubscription(a.editingID, a.subName, a.source)
				}
			})
		})
	})
}

func (a *application) subscriptionList(c *ui.Context, width float32) {
	ui.Column(c).Label(tr("订阅列表")).Gap(14).Children(func() {
		columns := max(1, int((width-48+12)/280))
		subs := append([]store.Subscription(nil), a.state.Subscriptions...)
		ui.Grid(c).Label(tr("订阅卡片网格")).Columns(columns).Gap(12).Children(func() {
			for i, sub := range subs {
				a.subscriptionCard(c.Key(sub.ID), sub, i, len(subs))
			}
		})
	})
}

func (a *application) subscriptionCard(c *ui.Context, sub store.Subscription, index, total int) {
	t := c.Theme()
	border, bg, titleColor := t.Border, paletteFor(t).Card, t.Text
	if a.editingID == sub.ID {
		border, bg, titleColor = selectionBorder(t, true), selectedBackground(t), t.Accent
	}
	ui.Column(c).Label(tr("订阅卡片 ")+sub.ID).Height(200).Padding(12).Radius(12).Background(bg).Border(0.5, border).Shadow(0, 2, 8, -2, ui.RGBA(0, 0, 0, 0.08)).Gap(4).Children(func() {
		if a.deletingID == sub.ID {
			ui.Text(c, tr("删除订阅？")).FontSize(15).Bold()
			ui.Text(c, sub.Name).TextColor(t.TextMuted).SingleLine()
			ui.Text(c, tr("同时移除该订阅的缓存频道，收藏记录保留。")).FontSize(11).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Row(c).Gap(10).Children(func() {
				if ui.Button(c, tr("确认删除")).Clicked() {
					a.deleteSubscription(sub.ID)
				}
				if ui.Button(c, tr("取消")).Clicked() {
					a.deletingID = ""
				}
			})
			return
		}
		ui.Row(c).Height(34).Shrink(0).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, sub.Name).FontSize(14).Bold().TextColor(titleColor).Grow(1).MinWidth(0).SingleLine().Tooltip(sub.Name)
			enabled := !sub.Disabled
			action, hint := tr("停用订阅 "), tr("当前已启用，点击停用此订阅")
			if sub.Disabled {
				action, hint = tr("启用订阅 "), tr("当前已停用，点击启用此订阅")
			}
			switcher := ui.Switch(c, &enabled).Label(action+sub.Name).Tooltip(hint).Size(36, 20).Radius(10)
			if enabled {
				switcher.Background(paletteFor(t).Success)
			}
			if enabled != !sub.Disabled {
				a.setSubscriptionEnabled(sub.ID, enabled)
			}
			if iconButton(c, tr("刷新订阅 ")+sub.Name, refreshIcon, false, sub.Disabled || sub.Source == "" || a.refreshing[sub.ID] != 0).Clicked() {
				a.refreshSubscription(sub.ID)
			}
			if iconButton(c, tr("删除订阅 ")+sub.Name, deleteIcon, false, false).Clicked() {
				a.deletingID = sub.ID
			}
		})
		location := tr("未绑定地址")
		if sub.Source != "" {
			location = subscriptionLocation(sub.Source)
		}
		hint, tooltip, color := location, location, t.TextMuted
		if sub.LastError != "" {
			hint = sub.LastError
			tooltip = location + "\n" + sub.LastError
			if len(sub.Channels) > 0 {
				tooltip += tr("\n已保留上次频道缓存")
			}
			color = t.Warning
		} else if a.refreshing[sub.ID] != 0 {
			hint = tr("正在更新…")
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, linkIcon).Size(13, 13).TextColor(color).Shrink(0)
			ui.Text(c, hint).FontSize(10).TextColor(color).Grow(1).MinWidth(0).SingleLine().Tooltip(tooltip)
		})
		updated := tr("最近更新：尚未更新")
		if !sub.UpdatedAt.IsZero() {
			updated = tr("最近更新：") + sub.UpdatedAt.Local().Format("2006-01-02 15:04")
		}
		ui.Row(c).Label(tr("订阅更新时间 ") + sub.ID).FillWidth().Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, calendarIcon).Size(13, 13).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, updated).FontSize(10).TextColor(t.TextMuted).Grow(1).MinWidth(0).SingleLine().Tooltip(updated)
		})
		ui.Spacer(c)
		ui.Row(c).Gap(4).Padding(7, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border.Alpha(.6)).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, tvIcon).Size(13, 13).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, fmt.Sprintf(tr("%d 个频道"), sub.ChannelCount)).FontSize(11).TextColor(t.TextMuted).Tooltip(updated).Grow(1).SingleLine()
			if settingsAction(c, tr("编辑订阅 ")+sub.Name, tr("编辑"), editIcon, false, false).Height(28).Padding(0, 8).Clicked() {
				a.editSubscription(sub.ID)
			}
			if iconButton(c, tr("前移订阅 ")+sub.Name, leftIcon, false, index == 0).Clicked() {
				a.moveSubscription(sub.ID, -1)
			}
			if iconButton(c, tr("后移订阅 ")+sub.Name, rightIcon, false, index == total-1).Clicked() {
				a.moveSubscription(sub.ID, 1)
			}
		})
		summary, detail := a.subscriptionEPGSummary(sub)
		ui.Row(c).Label(tr("订阅 EPG 统计 ") + sub.ID).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, calendarIcon).Size(13, 13).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, summary).FontSize(10).TextColor(t.TextMuted).Grow(1).MinWidth(0).SingleLine().Tooltip(summary + "\n" + detail)
		})
	})
}

func subscriptionLocation(source string) string {
	if u, err := url.Parse(source); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		u.User = nil
		u.RawQuery = ""
		u.Fragment = ""
		return u.String()
	}
	return source
}
