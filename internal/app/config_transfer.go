package app

import (
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"time"
)

func (a *application) backupSettingsPanel(c *ui.Context) {
	ui.Scroll(c.Key("backup-page")).Label(tr("备份页面")).TrackScroll(&a.backupScroll).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(24).Gap(16).Children(func() {
			a.configTransferPanel(c)
		})
	})
}

func (a *application) configTransferPanel(c *ui.Context) {
	t := c.Theme()
	warning := settingsWarning(t)
	blocked := a.busy || a.transferBusy || a.configError != nil
	ui.Column(c).Label(tr("配置备份与迁移")).Gap(12).Children(func() {
		ui.Text(c, tr("配置备份与迁移")).FontSize(16).Bold()
		ui.Row(c).Label(tr("备份操作分区")).Gap(16).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Label(tr("导出备份区域")).Grow(1).MinWidth(0).MinHeight(204).Padding(18).Gap(10).Radius(12).Background(panelBackground(t)).Border(1, t.Border).Children(func() {
				ui.Text(c, tr("导出备份")).FontSize(15).Bold()
				ui.Text(c, tr("保存网络订阅链接、收藏和设置。")).FontSize(12)
				ui.Text(c, tr("不含本地订阅、历史频道和频道缓存，导出前提示排除数量。JSON 可能含令牌，请妥善保管。")).FontSize(11).TextColor(t.TextMuted)
				a.settingsError(c, "export")
				ui.Spacer(c)
				if ui.Button(c, tr("导出配置…")).Disabled(blocked).Height(34).FillWidth().Clicked() {
					a.pickConfigExport()
				}
			})
			ui.Column(c).Label(tr("恢复配置区域")).Grow(1).MinWidth(0).MinHeight(204).Padding(18).Gap(10).Radius(12).Background(warning.Alpha(.05)).Border(1, warning.Alpha(.4)).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, tr("恢复配置")).FontSize(15).Bold().Grow(1)
					ui.Text(c, tr("覆盖操作")).FontSize(10).Bold().TextColor(warning)
				})
				ui.Text(c, tr("整份替换，不合并；成功后停止播放。")).FontSize(12).TextColor(warning)
				ui.Text(c, tr("仅支持当前 JSON 备份。先保留独立恢复快照；移除现有本地/历史条目，但不删除源文件。")).FontSize(11).TextColor(t.TextMuted)
				a.settingsError(c, "import")
				ui.Spacer(c)
				if ui.Button(c, tr("导入配置…")).Disabled(blocked).Height(34).FillWidth().Border(1, warning.Alpha(.5)).Clicked() {
					a.pickConfigImport()
				}
			})
		})
		if a.diagnosticError != nil || a.diagnostics.Err() != nil {
			ui.Text(c, tr("诊断日志写入失败，请检查配置目录的 logs/ 权限和磁盘空间。")).FontSize(11).TextColor(settingsDanger(t))
		}
		ui.Collapsible(c, tr("日志、快捷键与恢复说明"), &a.backupDetailsOpen, func() {
			if a.lastImportSnapshot != "" {
				ui.Column(c).Label(tr("恢复快照路径")).Children(func() {
					ui.Text(c, tr("最近一次导入前恢复快照：")+a.lastImportSnapshot).FontSize(11).TextColor(t.TextMuted)
				})
			}
			ui.Text(c, tr("恢复快照在 import-backups/ 中独立保留，不参与日常备份轮换；先关闭应用，再按 README 的方法恢复。首次建立配置没有旧快照。")).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, tr("脱敏日志：配置目录 logs/diagnostic.log，单文件最多 1 MiB，保留 3 个文件。仅记录源哈希和错误类别，不记录地址或请求头。")).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, tr("主界面快捷键：空格暂停/继续，上下切换当前筛选内的频道，左右调音量 ±5%。搜索栏展开和设置页禁用。视频持有焦点时需先点击原生界面；不注册全局快捷键。mpv 在非 Windows 暂不支持应用内暂停/音量。")).FontSize(11).TextColor(t.TextMuted)
		}).FillWidth()
	})
}
func (a *application) allowConfigTransfer() bool {
	if a.busy || a.transferBusy {
		return false
	}
	if a.configError != nil {
		a.status = tr("配置未恢复，无法导入或导出；请修复后重启")
		return false
	}
	return true
}
func (a *application) pickConfigExport() {
	if !a.allowConfigTransfer() {
		return
	}
	snapshot := a.state.Snapshot()
	a.transferBusy = true
	go func() {
		_, summary, err := store.PrepareExport(snapshot)
		if err != nil {
			a.diagnostics.Record(diagnostic.ConfigExport, "", err)
			a.update(func() {
				a.transferBusy = false
				a.status = tr("准备导出失败：") + err.Error()
				a.setSettingsError("export", a.status)
			})
			return
		}
		if summary.Local+summary.History > 0 {
			result, err := mygo.Dialog.Message(mygo.MessageOptions{
				Parent: a.window, Type: mygo.MessageWarning, Title: tr("导出范围确认"),
				Message: fmt.Sprintf(tr("仅导出 %d 个网络订阅、收藏和设置"), summary.Network),
				Detail:  fmt.Sprintf(tr("%d 个本地 M3U/TXT 订阅和 %d 个无链接历史订阅不会备份；不包含源文件或频道缓存。本地源请自行保管并在需要时重新导入。是否继续？"), summary.Local, summary.History),
				Buttons: []string{tr("取消"), tr("继续导出")}, DefaultButton: 0, CancelButton: 0,
			})
			if err != nil || result.Button != 1 {
				a.update(func() {
					a.transferBusy = false
					if err != nil {
						a.status = tr("导出确认失败：") + err.Error()
						a.setSettingsError("export", a.status)
					}
				})
				return
			}
		}
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.window, Title: tr("导出网络订阅和设置（不含本地订阅）"), DefaultPath: "LiuguangTV-backup.json", Filters: []mygo.FileFilter{{Name: tr("流光 TV JSON 备份"), Extensions: []string{"json"}}}})
		a.update(func() {
			if err != nil || path == "" {
				a.transferBusy = false
				if err != nil {
					a.status = tr("选择导出文件失败：") + err.Error()
					a.setSettingsError("export", a.status)
				}
				return
			}
			done := make(chan struct{})
			a.transferDone = done
			go func() {
				err := store.Export(path, a.path, snapshot)
				a.diagnostics.Record(diagnostic.ConfigExport, "", err)
				close(done)
				a.update(func() {
					a.transferBusy = false
					a.transferDone = nil
					if err != nil {
						a.status = tr("配置导出失败：") + err.Error()
						a.setSettingsError("export", a.status)
					} else {
						a.status = fmt.Sprintf(tr("已导出 %d 个网络订阅；已排除 %d 个本地、%d 个历史订阅；请勿公开分享备份"), summary.Network, summary.Local, summary.History)
						a.setSettingsError("export", "")
						a.showSettingsToast(a.status)
					}
				})
			}()
		})
	}()
}
func (a *application) pickConfigImport() {
	if !a.allowConfigTransfer() {
		return
	}
	a.transferBusy = true
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: tr("导入 JSON 备份"), Filters: []mygo.FileFilter{{Name: tr("流光 TV JSON 备份"), Extensions: []string{"json"}}}})
		if err != nil || len(paths) == 0 {
			a.update(func() {
				a.transferBusy = false
				if err != nil {
					a.status = tr("选择导入文件失败：") + err.Error()
					a.setSettingsError("import", a.status)
				}
			})
			return
		}
		state, err := store.ReadBackup(paths[0])
		if err != nil {
			a.diagnostics.Record(diagnostic.ConfigImport, "", err)
			a.update(func() {
				a.transferBusy = false
				a.status = tr("配置验证失败：") + err.Error()
				a.setSettingsError("import", a.status)
			})
			return
		}
		result, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: a.window, Type: mygo.MessageWarning, Title: tr("确认覆盖导入配置"), Message: fmt.Sprintf(tr("导入 %d 个订阅、%d 个收藏标记？"), len(state.Subscriptions), len(state.Favorites)), Detail: tr("将替换全部订阅、收藏和设置，不合并；原配置先保存到 import-backups/ 下独立、不会被日常保存轮换的恢复快照。写盘失败不会替换当前会话。导入成功会停止播放并重新拉取启用的网络订阅。当前本地订阅和历史频道也会从配置中移除（不删除其源文件），需要时请重新导入。备用 EPG 文件和 mpv 路径可能需调整。"), Buttons: []string{tr("取消"), tr("覆盖导入")}, DefaultButton: 0, CancelButton: 0})
		a.update(func() {
			if err != nil || result.Button != 1 {
				a.transferBusy = false
				if err != nil {
					a.status = tr("导入确认失败：") + err.Error()
					a.setSettingsError("import", a.status)
				}
				return
			}
			a.commitConfigImport(state)
		})
	}()
}

// Serialize replacement behind all admitted background writes. The UI is inert
// during the transaction; late old catalog results are rejected by sequence.
func (a *application) commitConfigImport(state store.State) {
	a.transferBusy = true
	a.configReplacing = true
	a.stateSequence++
	done := make(chan struct{})
	a.transferDone = done
	worker := a.background
	go func() {
		var err error
		var snapshot string
		if worker != nil {
			err = worker.Close()
		}
		if err == nil {
			snapshot, err = store.Import(a.path, state)
		}
		a.diagnostics.Record(diagnostic.ConfigImport, "", err)
		close(done)
		a.update(func() {
			a.transferBusy = false
			a.configReplacing = false
			a.transferDone = nil
			if snapshot != "" {
				a.lastImportSnapshot = snapshot
			}
			if err != nil {
				if worker != nil {
					a.startBackground()
					a.save()
				}
				a.status = tr("配置导入失败，当前会话未替换：") + err.Error()
				a.setSettingsError("import", a.status)
				return
			}
			a.installImportedConfig(state, worker != nil)
			a.status = tr("配置已导入；首次建立配置，无导入前恢复快照，本地路径请按需调整")
			if snapshot != "" {
				a.status = tr("配置已导入；原配置独立恢复快照：") + snapshot + tr("；本地路径请按需调整")
			}
			a.setSettingsError("import", "")
			message := tr("配置已导入；本地路径请按需调整")
			if snapshot != "" {
				message = tr("配置已导入；恢复快照可在备份页面查看")
			}
			a.showSettingsToast(message)
		})
	}()
}
func (a *application) installImportedConfig(state store.State, background bool) {
	a.stop()
	for id := range a.cancelLoads {
		a.cancelRefresh(id)
	}
	for _, job := range a.epg.jobs {
		if job.cancel != nil {
			job.cancel()
		}
	}
	if a.epg.calendarCancel != nil {
		a.epg.calendarCancel()
	}
	if a.epg.cancelCleanup != nil {
		a.epg.cancelCleanup()
	}
	a.epgGeneration++
	a.epg = epgState{calendarRequest: a.epg.calendarRequest + 1}
	a.state = state
	a.playerPathDraft, a.playerPathDraftLoaded = "", false
	a.player = nil // Re-select lazily after replacing engine preferences.
	if !a.webPlayback() {
		a.releaseWebPlayback()
	}
	a.catalogDirty = false
	a.favoritesRevision++
	a.group = "全部分组"
	a.query = ""
	a.showSearch = false
	a.cancelSubscriptionEdit()
	a.deletingID = ""
	a.ensureSelectedSource()
	a.installChannelIndex(indexCatalog(state))
	a.resetChannelList()
	if background {
		a.startBackground()
	}
	if a.window != nil {
		mygo.Theme.SetSource(nativeThemeSource(state.ThemeMode))
	}
	a.syncEPG(false)
	a.nextSubscriptionCheck = time.Time{}
}
