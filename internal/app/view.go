package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func (a *application) view(c *ui.Context) {
	c.SetTheme(themeForMode(a.state.ThemeMode, c.Theme().Dark))
	if a.cinema || a.windowCinema {
		c.SetTheme(appTheme)
	}
	t := c.Theme()
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	} else {
		c.Root().Background(t.Background)
	}
	if a.autoRefresh {
		c.After(time.Minute)
		if !time.Now().Before(a.nextSubscriptionCheck) {
			a.pollSubscriptions()
		}
	}
	if len(a.epg.jobs) > 0 || !a.epg.nextCleanup.IsZero() {
		c.After(time.Minute)
		if time.Now().After(a.epg.nextCheck) {
			a.pollEPG()
		}
	}
	if !a.transferBusy && c.Shortcut(0, ui.KeyF11) {
		a.toggleCinema()
	}
	if a.cinema && c.Shortcut(0, ui.KeyEscape) {
		a.toggleCinema()
	} else if a.windowCinema && c.Shortcut(0, ui.KeyEscape) {
		a.toggleWindowCinema()
	} else if a.showSearch && !a.showSettings && c.Shortcut(0, ui.KeyEscape) {
		a.toggleSearch()
	}
	barsVisible := a.fullscreenBarsVisible(time.Now())
	a.fullscreenControlsHovered = false
	if (a.cinema || a.windowCinema) && (a.fullscreenPointer != nil || (a.window != nil && fullscreenPointerSupported)) {
		c.After(100 * time.Millisecond)
	}
	// Native video follows the previous frame's layout, with event-driven settling.
	if a.host != nil && a.settle > 0 {
		a.settle--
		c.After(16 * time.Millisecond)
	}
	immersive := a.cinema || a.windowCinema
	overlay := immersive && embedhost.Supported
	root := ui.Column(c).Label(tr("应用界面")).Fill().Disabled(a.transferBusy)
	a.playbackShortcuts(c, root).Children(func() {
		if a.showSettings && !immersive {
			if a.host != nil {
				a.host.Show(false)
			}
			a.settingsPanel(c)
		} else {
			ui.Row(c).Label(tr("主分栏")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
				if !immersive {
					a.channelPanel(c)
				}
				ui.Column(c).Label(tr("播放面板")).Grow(1).MinWidth(0).MinHeight(0).Clip().Background(panelBackground(t)).Children(func() {
					if !immersive {
						a.playbackHeader(c)
					}
					videoBG := ui.Hex("#080808")
					if a.now == "" {
						videoBG = t.Background
					}
					viewport := ui.Column(c.Key("video-viewport")).Label(tr("视频画面")).Grow(1).FillWidth().MinHeight(100).Background(videoBG).Center().Gap(14).Children(func() {
						if a.webPlayback() && a.webView != nil && a.now != "" {
							ui.WebView(c, a.webView).Grow(1).FillWidth()
						} else if a.now == "" {
							ui.Column(c).Size(68, 68).Center().Radius(22).Background(selectedBackground(t)).Children(func() { ui.Icon(c, tvIcon).Size(32, 32).TextColor(t.Accent) })
							ui.Text(c, tr("视频播放区域")).FontSize(21).Bold()
							ui.Text(c, tr("选一个喜欢的频道，让精彩即刻开始")).FontSize(12).TextColor(t.TextMuted)
						}
					})
					r := viewport.Bounds()
					if !embedhost.Supported {
						a.videoViewport = r
					}
					if a.host != nil && !a.webPlayback() {
						if r.W > 0 && r.H > 0 {
							err := a.host.Place(r.X, r.Y, r.W, r.H)
							if err == nil {
								if overlay && barsVisible {
									controls := floatingControlsRect(r.W, r.H)
									err = a.host.SetOverlay(controls.X, controls.Y, controls.W, controls.H, floatingControlsRadius)
								} else {
									err = a.host.SetOverlay(0, 0, 0, 0, 0)
								}
							}
							if err != nil {
								a.status = err.Error()
								a.host.Show(false)
							} else {
								a.host.Show(a.now != "")
							}
						} else {
							a.host.Show(false)
						}
					}
					if barsVisible {
						a.playbackControls(c)
					}
				})
				if a.epg.show {
					a.epgPanel(c)
				}
			})
		}
		if a.showSettings && !immersive {
			a.settingsToastView(c)
		}
	})
}

func (a *application) playbackHeader(c *ui.Context) {
	bar := c.TitleBar()
	label, left, right := tr("沉浸顶栏"), float32(16), bar.Right+16
	height := max(56, bar.Height)
	width, _ := c.Size()
	contentWidth := width - 310 - left - right
	if a.epg.show {
		contentWidth -= 300
	}
	header := ui.Row(c).Label(label).Height(height).Shrink(0).Padding(0, right, 0, left).Gap(12).DragWindow()
	header.Material(glass.ScrollEdge{Hard: true, Background: c.Theme().Background})
	header.Children(func() {
		if a.now != "" {
			a.epgSummary(c)
		} else {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, tr("现在播放")).FontSize(16).Bold().SingleLine()
				ui.Text(c, tr("从频道库中发现你的下一段精彩")).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
			})
		}
		label := "READY"
		if a.now != "" {
			label = tr("连接中")
			if a.lineReady {
				label = "LIVE"
			}
		}
		if iconButton(c.Key("check-updates"), tr("检查更新"), updateIcon, false, false).Clicked() {
			checkForUpdates()
		}
		// Playback controls already show the state. In a narrow pane, give
		// the channel and program summary priority over the duplicate badge.
		if contentWidth >= 300 {
			badge(c, label, a.now != "")
		}
	})
}

func (a *application) channelPanel(c *ui.Context) {
	t := c.Theme()
	a.ensureChannelIndex()
	ui.Column(c).Label(tr("频道面板")).Width(310).Shrink(0).MinHeight(0).Clip().Padding(12).Background(sidebarBackground(c)).BorderWidth(0, 0.5, 0, 0).BorderColor(t.Border).Gap(12).Children(func() {
		ui.Row(c).Label(tr("频道标题")).Padding(4, 2, 4, max(2, c.TitleBar().Left-12)).DragWindow().Gap(6).Children(func() {
			ui.Image(c, appIcon).Label(tr("应用图标")).Size(28, 28).Shrink(0)
			a.sourceSelector(c)
			badge(c, fmt.Sprint(len(a.libraryIndex().channels)), false)
			if iconButton(c, tr("播放器设置"), settingsIcon, false, false).Clicked() {
				a.showSettings = true
				a.settle = 3
			}
		})
		ui.Row(c).Label(tr("频道筛选栏")).Gap(6).Children(func() {
			for _, tab := range []struct {
				label    string
				favorite bool
			}{{tr("全部频道"), false}, {tr("我的收藏"), true}} {
				b := ui.ButtonBase(c.Key(tab.favorite)).Label(tab.label).Grow(1).Height(32).Radius(8)
				bg, color := ui.Transparent, t.TextMuted
				selected := a.favoritesOnly == tab.favorite
				if selected {
					bg = selectedBackground(t)
					color = t.Accent
				} else if b.Hovered() {
					bg = t.SurfaceHover
				}
				b.Background(bg).Border(1, selectionBorder(t, selected)).Children(func() {
					text := ui.Text(c, tab.label).FontSize(12).TextColor(color)
					if selected {
						text.Bold()
					}
				})
				if b.Clicked() {
					a.favoritesOnly = tab.favorite
				}
			}
			label := tr("搜索")
			if a.showSearch {
				label = tr("关闭搜索")
			}
			if iconButton(c, label, searchIcon, a.showSearch, false).Clicked() {
				a.toggleSearch()
			}
		})
		if a.showSearch {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				ui.TextInput(c.Key("search"), &a.query).Label(tr("搜索频道")).Placeholder(tr("搜索频道…")).Grow(1).MinWidth(0).Height(34).AutoFocus()
				if a.query != "" && iconButton(c, tr("清除关键词"), closeIcon, false, false).Clicked() {
					a.query = ""
				}
			})
		} else {
			a.ensureChannelIndex()
			sel := ui.SelectBase(c, &a.group)
			trigger := sel.Trigger.Label(tr("频道分组")).Height(32).FillWidth().Padding(0, 14).Gap(8).Radius(t.Radius).Border(1, t.Border).Background(t.Surface)
			if trigger.Pressed() {
				trigger.Background(t.SurfacePressed)
			} else if trigger.Hovered() {
				trigger.Background(t.SurfaceHover)
			}
			trigger.Children(func() {
				ui.Text(c, groupTitle(a.group)).SingleLine().Grow(1).MinWidth(0)
				ui.Textf(c, tr("%d 个频道"), a.filteredChannels().Len()).FontSize(11).TextColor(t.TextMuted).Shrink(0)
				ui.Icon(c, downIcon).Size(12, 12).TextColor(t.TextMuted).Shrink(0)
			})
			sel.Popup(func(panel ui.Element) {
				panel.Margin(4, 0, 0, 0).Padding(4).Radius(t.Radius+2).Background(t.Background).Border(1, t.Border)
				ui.Scroll(c).MaxHeight(300).Children(func() {
					for _, group := range a.libraryIndex().groups {
						item := sel.Item(group).Label(tr("频道分组选项 ")+group).Padding(6, 10).Radius(t.Radius)
						if item.Highlighted() {
							item.Background(t.Accent).TextColor(t.AccentText)
						} else if group == a.group {
							item.Background(t.Surface)
						}
						item.Children(func() { ui.Text(c, groupTitle(group)).SingleLine() })
					}
				})
			})
		}
		filtered := a.filteredChannels()
		filterActive := strings.TrimSpace(a.query) != "" || a.group != "全部分组" || a.favoritesOnly
		if filtered.Len() == 0 {
			ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
				ui.Icon(c, tvIcon).Size(28, 28).TextColor(t.TextMuted)
				ui.Text(c, tr("暂无频道")).FontSize(16).Bold()
				if filterActive {
					ui.Text(c, tr("请调整搜索或筛选条件")).FontSize(11).TextColor(t.TextMuted)
				} else {
					ui.Text(c, tr("添加订阅源以导入频道")).FontSize(11).TextColor(t.TextMuted)
					if ui.Button(c, tr("管理订阅源")).Clicked() {
						a.settingsEPGTab = false
						a.settingsPlayerTab = false
						a.settingsAppearanceTab = false
						a.settingsBackupTab = false
						a.settingsAboutTab = false
						a.showSettings = true
						a.settle = 3
					}
				}
			})
		} else {
			a.list.Key = func(i int) any { return playlist.ChannelKey(filtered.At(i)) }
			ui.List(c, &a.list, filtered.Len(), func(i int) {
				ch := filtered.At(i)
				active := a.now != "" && playlist.ChannelKey(a.activeChannel) == playlist.ChannelKey(ch)
				bg := ui.Transparent
				if active {
					bg = selectedBackground(t)
				}
				ui.Row(c).Label(tr("频道行 ")+playlist.ChannelKey(ch)).Height(54).Shrink(0).Padding(4).Radius(10).Background(bg).Border(1, selectionBorder(t, active)).Gap(4).Draw(func(p *ui.Painter, r ui.Rect) {
					if !active && i+1 < filtered.Len() {
						p.Line(r.X+50, r.Y+r.H-.5, r.X+r.W-8, r.Y+r.H-.5, .5, t.Border.Alpha(.6))
					}
				}).Children(func() {
					b := ui.ButtonBase(c.Key("channel")).Label(tr("播放 ") + ch.Name).Tooltip(ch.Name).Grow(1).MinWidth(0).FillHeight().Radius(8).Justify(ui.Start).Gap(10).Padding(4)
					if b.Hovered() && !active {
						b.Background(t.SurfaceHover)
					}
					b.Children(func() {
						iconColor, iconBG := t.TextMuted, t.Surface
						if active {
							iconColor = t.Accent
							iconBG = paletteFor(t).ActiveIcon
						}
						ui.Column(c).Size(32, 32).Shrink(0).Center().Radius(9).Background(iconBG).Children(func() { ui.Icon(c, tvIcon).Size(16, 16).TextColor(iconColor) })
						ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
							color := t.Text
							if active {
								color = t.Accent
							}
							ui.Row(c).FillWidth().Gap(6).Children(func() {
								ui.Text(c, ch.Name).FontSize(12).TextColor(color).Bold().SingleLine().Grow(1).MinWidth(0)
								if active {
									ui.Text(c, tr("播放中")).FontSize(10).TextColor(t.Accent).Shrink(0)
								}
							})
							summary := ch.Group
							if count := len(playlist.Lines(ch)); count > 1 {
								summary += fmt.Sprintf(tr(" · %d 条线路"), count)
							}
							if p, _ := epg.NowNext(a.schedule(ch), time.Now()); p != nil {
								summary = tr("正在播出：") + p.Title
							}
							ui.Text(c, summary).FontSize(10).TextColor(t.TextMuted).SingleLine()
						})
					})
					if b.Clicked() {
						a.play(ch)
					}
					label := tr("收藏")
					if a.isFavorite(ch) {
						label = tr("取消收藏")
					}
					if iconButton(c.Key("favorite"), label, starIcon, a.isFavorite(ch), false).Clicked() {
						a.toggleFavorite(ch)
					}
				})
			}).Label(tr("频道列表")).Grow(1).MinHeight(0).Clip().Gap(4)
		}
	})
}

func (a *application) playbackControls(c *ui.Context) {
	t := c.Theme()
	immersive := a.cinema || a.windowCinema
	width, _ := c.Size()
	if !immersive {
		width -= 310
	}
	if a.epg.show {
		width -= 300
	}
	availableWidth := width
	if immersive {
		width = floatingControlsRect(width, 0).W
	}
	compact := width < 600
	height, padding, gap := float32(52), float32(12), float32(8)
	if immersive {
		height, padding, gap = floatingControlsHeight, 16, 10
	}
	bar := ui.Row(c).Label(tr("播放控制栏")).Height(height).Shrink(0).Padding(0, padding).Gap(gap)
	if !immersive {
		bar.Background(t.Surface).BorderWidth(0.5, 0, 0, 0).BorderColor(t.Border)
	}
	if immersive {
		bar.Width(width).AlignSelf(ui.Center).Radius(floatingControlsRadius)
		if embedhost.Supported {
			bar.Absolute().Bottom(floatingControlsBottom).Left((availableWidth - width) / 2)
		} else {
			bar.Margin(0, 0, floatingControlsBottom, 0)
		}
		bar.Background(ui.Hex("#161616")).Border(0.5, ui.Hex("#363639"))
		a.fullscreenControlsHovered = a.fullscreenControlsHovered || bar.Hovered()
	}
	bar.Children(func() {
		label, icon := tr("暂停"), pauseIcon
		if a.controls.Paused {
			label, icon = tr("继续"), playIcon
		}
		if iconButton(c, label, icon, true, a.now == "" || !a.supportsPlaybackControls()).Clicked() {
			a.togglePause()
		}
		if immersive || width >= 800 {
			empty := a.filteredChannels().Len() == 0
			if iconButton(c, tr("上一频道"), previousChannelIcon, false, empty).Clicked() {
				a.stepChannel(-1)
			}
			if iconButton(c, tr("下一频道"), nextChannelIcon, false, empty).Clicked() {
				a.stepChannel(1)
			}
		}
		if iconButton(c, tr("停止"), stopIcon, false, a.now == "").Clicked() {
			a.stop()
		}
		state := tr("待播放")
		if a.now != "" {
			state = tr("连接中")
			if a.lineReady {
				state = "LIVE"
			}
		}
		if a.controls.Paused && a.now != "" {
			state = tr("已暂停")
		}
		badge(c, state, a.now != "")
		a.statusHint(c).Grow(1).MinWidth(0)
		if compact {
			iconButton(c, tr("更多播放操作"), settingsIcon, false, false).Menu(a.morePlaybackMenu)
		} else {
			if labels := lineLabels(a.activeChannel); len(labels) > 1 {
				// Element.Menu uses the OS popup menu. A GPU-painted Select popup
				// cannot appear above the separate native mpv child HWND.
				button := ui.ButtonBase(c.Key("line-selector")).Label(tr("播放线路")).Tooltip(a.selectedLine).Width(145).Height(32).Shrink(0).Padding(0, 10).Radius(9).Border(1, t.Border).Background(t.Surface).Gap(6)
				if button.Hovered() {
					button.Background(t.SurfaceHover)
				}
				button.Children(func() {
					ui.Text(c, a.selectedLine).FontSize(12).Grow(1).MinWidth(0).SingleLine()
					ui.Icon(c, downIcon).Size(12, 12).Shrink(0).TextColor(t.TextMuted)
				})
				button.Menu(a.playbackLineMenu)
			}
			label, icon = tr("静音"), soundIcon
			if a.controls.Muted {
				label, icon = tr("取消静音"), muteIcon
			}
			if iconButton(c, label, icon, a.controls.Muted, !a.supportsPlaybackControls()).Clicked() {
				a.controls.Muted = !a.controls.Muted
				a.syncPlaybackControls()
			}
			if ui.Slider(c, &a.controls.Volume, 0, 100).Label(tr("音量")).Width(92).Disabled(!a.supportsPlaybackControls()).Changed() {
				a.syncPlaybackControls()
			}
			ui.Textf(c, "%.0f%%", a.controls.Volume).Width(32).FontSize(11).TextColor(t.TextMuted)
		}
		label = tr("窗口全屏")
		windowMode := a.windowCinema && !a.cinema
		if windowMode {
			label = tr("退出窗口全屏")
		}
		if iconButton(c, label, windowExpandIcon, windowMode, false).Clicked() {
			a.toggleWindowCinema()
		}
		label = tr("全屏")
		if a.cinema {
			label = tr("退出全屏")
		}
		if iconButton(c, label, expandIcon, a.cinema, false).Clicked() {
			a.toggleCinema()
		}
	})
}
