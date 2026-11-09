package app

import (
	"fmt"
	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"github.com/tangqiu0205/LiuguangTV/internal/l10n"

	"github.com/tangqiu0205/LiuguangTV/internal/statework"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func (a *application) startBackground() {
	a.ensureSelectedSource()
	a.installChannelIndex(indexCatalog(a.state))
	a.background = statework.New(a.path, func(result statework.Result) {
		var index channelIndex
		if result.Build {
			index = indexCatalog(result.State)
		}
		a.update(func() { a.applyBackground(result, index) })
	})
}
func (a *application) catalogChanged() {
	a.syncEPG(false)
	a.ensureSelectedSource()
	a.catalogDirty = true
	a.save()
}
func (a *application) save() {
	// A confirmed import owns the writer until its commit completes.
	if a.configReplacing {
		return
	}
	if a.configError != nil {
		a.status = tr("配置未恢复，保存已禁用；修复配置后请重启：") + a.configError.Error()
		return
	}
	if a.background == nil {
		// Headless callers can execute deterministically without an event loop.
		if a.catalogDirty {
			a.state.RebuildChannels()
			a.catalogDirty = false
			a.installChannelIndex(indexCatalog(a.state))
			a.resetChannelList()
		}
		if err := store.Save(a.path, a.state); err != nil {
			a.saveError = tr("保存设置失败：") + err.Error()
			a.status = a.saveError
		} else {
			a.saveError = ""
			a.finishRefreshStatus()
			a.finishSettingsSave()
		}
		return
	}
	a.stateSequence++
	a.background.Submit(statework.Request{Sequence: a.stateSequence, State: a.state.Snapshot(), Build: a.catalogDirty})
}
func (a *application) applyBackground(result statework.Result, index channelIndex) {
	if result.Sequence != a.stateSequence || a.closed {
		return
	}
	if result.Build {
		a.state.Channels = result.State.Channels
		a.state.FavoriteURLs = result.State.FavoriteURLs
		a.state.SourceChannels = result.State.SourceChannels
		a.state.Favorites = result.State.Favorites
		for i, sub := range result.State.Subscriptions {
			if i < len(a.state.Subscriptions) && a.state.Subscriptions[i].ID == sub.ID {
				a.state.Subscriptions[i].ChannelCount = sub.ChannelCount
			}
		}
		a.catalogDirty = false
		a.favoritesRevision++
		a.installChannelIndex(index)
		a.resetChannelList()
	}
	a.diagnostics.Record(diagnostic.StateSave, "", result.Err)
	if result.Err != nil {
		a.saveError = tr("保存设置失败：") + result.Err.Error()
		a.status = a.saveError
	} else {
		a.saveError = ""
		a.finishRefreshStatus()
		a.finishSettingsSave()
	}
}
func (a *application) finishRefreshStatus() {
	if !l10n.Matches(a.status, "订阅更新完成，正在整理频道…") {
		return
	}
	result := a.refreshResult
	a.refreshResult = subscriptionRefreshResult{}
	if result.completed == 0 {
		return
	}
	if result.completed == 1 {
		if result.failed != 0 {
			a.status = fmt.Sprintf(tr("订阅 %s 更新失败，保留上次频道缓存"), result.name)
			return
		}
		if sub := a.state.FindSubscription(result.id); sub != nil && !sub.Disabled {
			a.status = fmt.Sprintf(tr("订阅 %s 更新完成，共 %d 个频道"), result.name, sub.ChannelCount)
		} else {
			a.status = fmt.Sprintf(tr("订阅 %s 更新完成"), result.name)
		}
	} else if result.failed != 0 {
		a.status = fmt.Sprintf(tr("本次更新 %d 个订阅，%d 个失败（保留上次频道缓存）"), result.completed, result.failed)
		return
	} else {
		a.status = fmt.Sprintf(tr("本次更新 %d 个订阅，全部成功"), result.completed)
	}
	a.showSettingsToast(a.status)
}
