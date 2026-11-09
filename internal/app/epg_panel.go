package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func (a *application) prepareEPGCalendar(now time.Time) {
	ch := a.playingEPGChannel()
	ps := a.scheduleAt(ch, now)
	var first *epg.Programme
	if len(ps) > 0 {
		first = &ps[0]
	}
	channel, today := playlist.ChannelKey(ch)+"|source:"+ch.SourceID, now.Format("2006-01-02")
	channelChanged := a.epg.calendarChannel != channel || a.epg.calendarToday != today
	if !channelChanged && a.epg.calendarFirst == first && a.epg.calendarCount == len(ps) {
		return
	}
	if a.epg.calendarCancel != nil {
		a.epg.calendarCancel()
	}
	a.epg.calendarRequest++
	token := a.epg.calendarRequest
	a.epg.calendarFirst, a.epg.calendarCount = first, len(ps)
	a.epg.calendarChannel, a.epg.calendarToday = channel, today
	if channelChanged {
		a.epg.calendar = epg.Calendar{}
		a.epg.date = ""
		a.epg.listKey = ""
	}
	if a.window == nil && a.dispatch == nil {
		a.applyEPGCalendar(token, epg.CalendarFor(ps, now), today)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.epg.calendarCancel = cancel
	a.epg.calendarLoading = true
	go func() {
		defer cancel()
		calendar, err := epg.CalendarForContext(ctx, ps, now)
		if err == nil {
			a.update(func() { a.applyEPGCalendar(token, calendar, today) })
		}
	}()
}
func (a *application) applyEPGCalendar(token uint64, calendar epg.Calendar, today string) {
	if token != a.epg.calendarRequest {
		return
	}
	a.epg.calendar = calendar
	a.epg.calendarLoading = false
	a.epg.calendarCancel = nil
	a.epg.calendarVersion++
	if len(a.epg.calendar.Days[a.epg.date]) == 0 {
		a.epg.date = ""
		dates := a.epg.calendar.Dates
		if len(dates) > 0 {
			index := sort.SearchStrings(dates, today)
			if index == len(dates) {
				index--
			}
			a.epg.date = dates[index]
		}
	}
}
func (a *application) closeEPGPanel() {
	a.epg.show = false
	if a.epg.calendarCancel != nil {
		a.epg.calendarCancel()
		a.epg.calendarCancel = nil
	}
	a.epg.calendarRequest++
	a.epg.calendarLoading = false
	a.epg.calendar = epg.Calendar{}
	a.epg.calendarFirst = nil
	a.epg.calendarChannel = ""
	a.epg.list = ui.ListState{}
	a.epg.listKey = ""
	a.settle = 3
}
func epgDateLabel(date string, now time.Time) string {
	if date == now.Format("2006-01-02") {
		return tr("今天 · ") + date[5:]
	}
	if date == now.AddDate(0, 0, -1).Format("2006-01-02") {
		return tr("昨天 · ") + date[5:]
	}
	if date == now.AddDate(0, 0, 1).Format("2006-01-02") {
		return tr("明天 · ") + date[5:]
	}
	return date
}
func (a *application) selectEPGDate(date string) {
	if len(a.epg.calendar.Days[date]) > 0 {
		a.epg.date = date
	}
}
func (a *application) epgPanel(c *ui.Context) {
	t := c.Theme()
	now := time.Now()
	a.prepareEPGCalendar(now)
	ui.Column(c).Label(tr("EPG 节目单")).Width(300).Shrink(0).MinHeight(0).Background(panelBackground(t)).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Padding(14).Gap(12).Children(func() {
		ui.Text(c, tr("节目单")).FontSize(16).Bold()
		ui.Text(c, a.now).FontSize(12).TextColor(t.TextMuted).SingleLine()
		dates := a.epg.calendar.Dates
		if len(dates) > 0 {
			index := sort.SearchStrings(dates, a.epg.date)
			ui.Row(c).Label(tr("节目日期导航")).Gap(6).Shrink(0).Children(func() {
				if ui.Button(c, "‹").Label(tr("上一节目日期")).Tooltip(tr("上一个有节目的日期")).Width(30).Height(30).Disabled(index <= 0).Clicked() {
					a.selectEPGDate(dates[index-1])
				}
				button := ui.Button(c, epgDateLabel(a.epg.date, now)).Label(tr("节目日期")).Grow(1).MinWidth(0).Height(30)
				// Use a native menu: ordinary GPU popups can sit behind the mpv HWND.
				button.Menu(func(menu *ui.Menu) {
					for _, date := range dates {
						if menu.Item(epgDateLabel(date, now)).Checked(date == a.epg.date).Chosen() {
							a.selectEPGDate(date)
						}
					}
				})
				if ui.Button(c, "›").Label(tr("下一节目日期")).Tooltip(tr("下一个有节目的日期")).Width(30).Height(30).Disabled(index >= len(dates)-1).Clicked() {
					a.selectEPGDate(dates[index+1])
				}
			})
		}
		rows := a.epg.calendar.Days[a.epg.date]
		key := fmt.Sprintf("%d|%s", a.epg.calendarVersion, a.epg.date)
		if a.epg.listKey != key {
			a.epg.list = ui.ListState{}
			row := max(0, a.epg.calendar.CurrentRow(a.epg.date, now))
			a.epg.list.ScrollTo(row, ui.Center)
			a.epg.listKey = key
		}
		if len(rows) == 0 {
			text := tr("暂无节目信息")
			if a.epg.calendarLoading {
				text = tr("正在整理节目单…")
			}
			ui.Column(c).Grow(1).Center().Children(func() { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted) })
		} else {
			a.epg.list.Key = func(i int) any {
				p := a.epg.calendar.At(a.epg.date, i)
				return fmt.Sprintf("%d|%d|%s|%d", p.Start.Unix(), p.Stop.Unix(), p.Title, i)
			}
			ui.List(c, &a.epg.list, len(rows), func(i int) {
				p := a.epg.calendar.At(a.epg.date, i)
				active := !now.Before(p.Start) && now.Before(p.Stop)
				bg, color := t.Surface, t.Text
				if active {
					bg = selectedBackground(t)
					color = t.Accent
				}
				ui.Column(c).Label(tr("节目行")).FillWidth().Height(66).Shrink(0).Padding(10).Radius(8).Background(bg).Border(1, selectionBorder(t, active)).Gap(5).Children(func() {
					stamp := p.Start.Local().Format("15:04") + "–" + p.Stop.Local().Format("15:04")
					if p.Stop.Local().Format("2006-01-02") != p.Start.Local().Format("2006-01-02") {
						stamp = p.Start.Local().Format("01-02 15:04") + "–" + p.Stop.Local().Format("01-02 15:04")
					}
					if active {
						stamp += tr(" · 正在播出")
					}
					ui.Text(c, stamp).FontSize(10).TextColor(t.TextMuted).SingleLine().Tooltip(stamp)
					ui.Text(c, p.Title).FontSize(12).Bold().TextColor(color).SingleLine().Tooltip(p.Description)
				})
			}).Label(tr("节目列表")).Grow(1).MinHeight(0).Clip().Gap(8)
		}
		if ui.Button(c, tr("关闭节目单")).FillWidth().Shrink(0).Clicked() {
			a.closeEPGPanel()
		}
	})
}
