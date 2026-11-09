package app

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/loadqueue"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

type subscriptionRefreshResult struct {
	completed, failed int
	id, name          string
}

func (a *application) cancelRefresh(id string) {
	if cancel := a.cancelLoads[id]; cancel != nil {
		cancel()
	}
	delete(a.cancelLoads, id)
	delete(a.refreshing, id)
	if len(a.refreshing) == 0 {
		a.refreshResult = subscriptionRefreshResult{}
	}
}
func (a *application) saveSubscription(id, name, source string) {
	if a.subEPG {
		a.saveManualEPG(a.editingEPGID, name, source)
		return
	}
	savedID, err := a.state.UpsertSubscription(id, name, source)
	if err != nil {
		a.status = err.Error()
		a.setSettingsError("subscription-editor", err.Error())
		return
	}
	a.setSettingsError("subscription-editor", "")
	a.cancelRefresh(savedID)
	a.editingID = ""
	a.showSubEditor = false
	a.subName = ""
	a.source = ""
	a.deletingID = ""
	a.status = tr("订阅已保存，正在更新…")
	if a.state.FindSubscription(savedID).Disabled {
		a.status = tr("订阅已保存（已停用）")
	}
	a.pendingSettingsSuccess = tr("订阅已保存")
	a.catalogChanged()
	a.refreshSubscription(savedID)
}
func (a *application) cancelSubscriptionEdit() {
	a.setSettingsError("subscription-editor", "")
	a.editingID = ""
	a.subName = ""
	a.source = ""
	a.showSubEditor = false
	a.subEPG = false
	a.editingEPGID = ""
}

func (a *application) editSubscription(id string) {
	if sub := a.state.FindSubscription(id); sub != nil {
		a.subEPG = false
		a.editingEPGID = ""
		a.editingID = id
		a.showSubEditor = true
		a.subName = sub.Name
		a.source = sub.Source
		a.deletingID = ""
		a.editorScroll.ScrollTo(0, 0, ui.Animation{})
	}
}
func (a *application) deleteSubscription(id string) {
	a.cancelRefresh(id)
	if a.state.RemoveSubscription(id) {
		if a.editingID == id {
			a.editingID = ""
			a.showSubEditor = false
			a.subName = ""
			a.source = ""
		}
		a.deletingID = ""
		a.status = tr("订阅已删除；收藏记录保留")
		a.pendingSettingsSuccess = a.status
		a.catalogChanged()
	}
}
func (a *application) setSubscriptionEnabled(id string, enabled bool) {
	a.cancelRefresh(id)
	if !a.state.UpdateSubscriptionEnabled(id, enabled) {
		return
	}
	a.status = tr("订阅已停用，缓存保留")
	if enabled {
		a.status = tr("订阅已启用")
	}
	a.pendingSettingsSuccess = a.status
	a.catalogChanged()
	if enabled {
		a.refreshSubscription(id)
	}
}
func (a *application) moveSubscription(id string, delta int) {
	if a.state.ReorderSubscription(id, delta) {
		a.status = tr("订阅顺序已保存")
		a.pendingSettingsSuccess = a.status
		a.catalogChanged()
	}
}

func (a *application) resetChannelList() {
	a.ensureChannelIndex()
	if !a.libraryIndex().groupSet[a.group] {
		a.group = "全部分组"
	}
	a.list = ui.ListState{}
}
func (a *application) refreshAll() {
	count := 0
	for _, sub := range a.state.Subscriptions {
		if sub.Source != "" && !sub.Disabled {
			a.refreshSubscription(sub.ID)
			count++
		}
	}
	if count > 0 {
		a.status = fmt.Sprintf(tr("正在更新 %d 个订阅，缓存频道仍可播放…"), count)
	}
}
func (a *application) refreshSubscription(id string) {
	sub := a.state.FindSubscription(id)
	if sub == nil || sub.Disabled || sub.Source == "" || a.refreshing[id] != 0 {
		return
	}
	if len(a.refreshing) == 0 {
		a.refreshResult = subscriptionRefreshResult{}
	}
	if a.refreshing == nil {
		a.refreshing = map[string]uint64{}
		a.cancelLoads = map[string]context.CancelFunc{}
		a.loadSlots = make(chan struct{}, 3)
	}
	a.loadSequence++
	token := a.loadSequence
	ctx, cancel := context.WithCancel(context.Background())
	a.refreshing[id] = token
	a.cancelLoads[id] = cancel
	source, revision := sub.Source, sub.Revision
	slots := a.loadSlots
	a.status = tr("正在更新订阅：") + sub.Name
	go func() {
		defer cancel()
		var doc playlist.Document
		var err error
		request, release, err := loadqueue.Acquire(ctx, slots, 30*time.Second)
		if err == nil {
			doc, err = playlist.LoadDocument(request, source, a.diagnostics)
			release()
		}
		a.update(func() {
			if a.refreshing[id] == token && err == nil {
				if sub := a.state.FindSubscription(id); sub != nil && sub.Revision == revision && !sub.Disabled {
					sub.EPGURLs = doc.EPGURLs
				}
			}
			a.completeRefresh(id, token, revision, doc.Channels, err)
		})
	}()
}
func (a *application) completeRefresh(id string, token, revision uint64, channels []playlist.Channel, err error) {
	if a.refreshing[id] != token {
		return
	}
	delete(a.refreshing, id)
	delete(a.cancelLoads, id)
	if !a.state.UpdateSubscriptionCache(id, revision, channels, err, time.Now()) {
		return
	}
	a.refreshResult.completed++
	a.refreshResult.id = id
	a.refreshResult.name = a.state.FindSubscription(id).Name
	if err != nil {
		a.refreshResult.failed++
	}
	if len(a.refreshing) > 0 {
		a.status = fmt.Sprintf(tr("正在更新订阅，剩余 %d 个…"), len(a.refreshing))
	} else {
		a.status = tr("订阅更新完成，正在整理频道…")
	}
	if err == nil {
		a.catalogChanged()
	} else {
		a.save()
	}
}

// File selection only changes the draft; saving is an explicit editor action.
func (a *application) pickSubscriptionFile() {
	if a.busy {
		return
	}
	a.busy = true
	title := tr("选择本地频道列表")
	filter := mygo.FileFilter{Name: tr("M3U / M3U8 / TXT 频道列表"), Extensions: []string{"m3u", "m3u8", "txt"}}
	if a.subEPG {
		title = tr("选择本地 EPG 节目单")
		filter = mygo.FileFilter{Name: tr("XMLTV 节目单"), Extensions: []string{"xml", "gz"}}
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: title, Filters: []mygo.FileFilter{filter}})
		a.update(func() {
			a.busy = false
			if err != nil {
				a.status = tr("打开文件失败：") + err.Error()
				a.setSettingsError("subscription-editor", a.status)
				return
			}
			if len(paths) == 0 {
				return
			}
			a.setSettingsError("subscription-editor", "")
			a.source = paths[0]
		})
	}()
}
