package app

import "github.com/egoist/mygo/ui"

func (a *application) saveManualEPG(id, name, source string) {
	if a.configError != nil {
		a.status = tr("配置未恢复，EPG 设置未保存；修复配置后请重启：") + a.configError.Error()
		a.setSettingsError("subscription-editor", a.status)
		return
	}
	if _, err := a.state.UpsertEPGSource(id, name, source); err != nil {
		a.status = err.Error()
		a.setSettingsError("subscription-editor", a.status)
		return
	}
	a.cancelSubscriptionEdit()
	a.settingsEPGTab = true
	a.status = tr("EPG 来源已提交保存，正在加载节目单…")
	a.pendingSettingsSuccess = tr("EPG 来源已保存")
	a.save()
	a.syncEPG(false)
}

func (a *application) editManualEPG(id string) {
	for _, entry := range a.state.ManualEPGSources() {
		if entry.ID == id {
			a.cancelSubscriptionEdit()
			a.showSubEditor, a.subEPG = true, true
			a.editingEPGID, a.subName, a.source = id, entry.Name, entry.Source
			a.deletingEPGID = ""
			a.editorScroll.ScrollTo(0, 0, ui.Animation{})
			return
		}
	}
}

func (a *application) moveManualEPG(id string, delta int) {
	if a.configError != nil {
		a.status = tr("配置未恢复，无法调整 EPG 顺序")
		return
	}
	if a.state.MoveEPGSource(id, delta) {
		a.status = tr("EPG 顺序已提交保存")
		a.pendingSettingsSuccess = tr("EPG 顺序已保存")
		a.save()
	}
}

func (a *application) deleteManualEPG(id string) {
	if a.configError != nil {
		a.status = tr("配置未恢复，无法删除 EPG 来源")
		return
	}
	if !a.state.RemoveEPGSource(id) {
		return
	}
	if a.editingEPGID == id {
		a.cancelSubscriptionEdit()
	}
	a.deletingEPGID = ""
	a.status = tr("EPG 来源已提交删除")
	a.pendingSettingsSuccess = tr("EPG 来源已删除")
	a.save()
	a.syncEPG(false)
}
