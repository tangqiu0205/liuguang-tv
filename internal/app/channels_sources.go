package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *application) ensureSelectedSource() {
	if a.state.SelectedSource == "" {
		return
	}
	sub := a.state.FindSubscription(a.state.SelectedSource)
	if sub == nil || sub.Disabled {
		a.state.SelectedSource = ""
		a.channelCache.valid = false
	}
}
func (a *application) sourceTitle() string {
	if sub := a.state.FindSubscription(a.state.SelectedSource); sub != nil && !sub.Disabled {
		return sub.Name
	}
	return tr("频道库")
}
func (a *application) selectLibrarySource(id string) {
	if id != "" {
		sub := a.state.FindSubscription(id)
		if sub == nil || sub.Disabled {
			return
		}
	}
	if a.state.SelectedSource == id {
		return
	}
	a.state.SelectedSource = id
	a.group = "全部分组"
	a.query = ""
	a.channelCache.valid = false
	a.list = ui.ListState{}
	a.save()
}
func (a *application) sourceSelector(c *ui.Context) {
	t := c.Theme()
	title := a.sourceTitle()
	b := ui.ButtonBase(c.Key("source-selector")).Label(tr("切换订阅源")).Tooltip(tr("切换订阅源 · ")+title).Grow(1).MinWidth(0).Height(44).Padding(0, 4).Radius(7).Justify(ui.Start).Gap(7)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, tr("当前播放列表")).FontSize(10).TextColor(t.TextMuted).SingleLine()
			ui.Text(c, title).FontSize(15).Bold().SingleLine().MinWidth(0)
		})
		ui.Icon(c, downIcon).Size(12, 12).Shrink(0).TextColor(t.TextMuted)
	})
	b.Menu(func(menu *ui.Menu) {
		if menu.Item(tr("全部订阅")).Checked(a.state.SelectedSource == "").Chosen() {
			a.selectLibrarySource("")
		}
		menu.Separator()
		for i, sub := range a.state.Subscriptions {
			label := fmt.Sprintf("%d. %s", i+1, sub.Name)
			if sub.Disabled {
				label += tr("（已停用）")
			}
			if menu.Item(label).Checked(a.state.SelectedSource == sub.ID).Disabled(sub.Disabled).Chosen() {
				a.selectLibrarySource(sub.ID)
			}
		}
	})
}
