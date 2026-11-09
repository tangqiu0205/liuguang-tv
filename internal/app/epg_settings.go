package app

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

func cleanupResult(count int, err error) string {
	if err != nil {
		return tr("缓存清理失败，请检查目录权限")
	}
	return fmt.Sprintf(tr("缓存清理完成，删除 %d 个过期文件"), count)
}
func epgSourceID(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source)))[:8] }
func epgSourceDisplay(source string) string {
	if u, err := url.Parse(source); err == nil && u.Host != "" {
		return u.Hostname() + " · " + epgSourceID(source)
	}
	return tr("本地文件 · ") + filepath.Base(source)
}

func (a *application) epgSettings(c *ui.Context, width float32) {
	t := c.Theme()
	entries := a.state.ManualEPGSources()
	if len(entries) == 0 {
		ui.Column(c).Label(tr("EPG 空状态")).Grow(1).MinHeight(0).Center().Gap(12).Children(func() {
			ui.Icon(c, calendarIcon).Size(36, 36).TextColor(t.TextMuted)
			ui.Text(c, tr("尚未添加节目单 EPG")).FontSize(15).TextColor(t.TextMuted)
		})
		return
	}
	ui.Grid(c).Label(tr("EPG 卡片网格")).Columns(max(1, int((width-48+12)/280))).Gap(12).Children(func() {
		for index, entry := range entries {
			source, id := entry.Source, epgSourceID(entry.Source)
			job := a.epg.jobs[source]
			if job == nil {
				job = &epgJob{}
			}
			border, background, titleColor := t.Border, paletteFor(t).Card, t.Text
			if a.editingEPGID == entry.ID {
				border, background, titleColor = selectionBorder(t, true), selectedBackground(t), t.Accent
			}
			ui.Column(c.Key(entry.ID)).Label(tr("EPG 源 ")+id).MinHeight(200).Padding(12).Radius(12).Background(background).Border(.5, border).Shadow(0, 2, 8, -2, ui.RGBA(0, 0, 0, .08)).Gap(6).Children(func() {
				if a.deletingEPGID == entry.ID {
					ui.Text(c, tr("删除节目单？")).FontSize(15).Bold()
					ui.Text(c, entry.Name).FontSize(12).SingleLine()
					ui.Text(c, tr("仅移除此手动来源，不删除订阅自带节目单。")).FontSize(11).TextColor(t.TextMuted)
					ui.Spacer(c)
					ui.Row(c).Gap(8).Children(func() {
						if ui.Button(c, tr("确认删除")).Label(tr("确认删除 EPG ") + entry.ID).Clicked() {
							a.deleteManualEPG(entry.ID)
						}
						if ui.Button(c, tr("取消")).Clicked() {
							a.deletingEPGID = ""
						}
					})
					return
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, calendarIcon).Size(18, 18).TextColor(t.Accent).Shrink(0)
					ui.Text(c, entry.Name).FontSize(14).Bold().TextColor(titleColor).Grow(1).MinWidth(0).SingleLine().Tooltip(entry.Name)
					if iconButton(c, tr("刷新 EPG 源 ")+id, refreshIcon, false, job.loading).Clicked() {
						a.loadEPG(source)
					}
					if iconButton(c, tr("删除 EPG ")+entry.ID, deleteIcon, false, false).Clicked() {
						a.deletingEPGID = entry.ID
					}
				})
				ui.Text(c, epgSourceDisplay(source)).FontSize(11).TextColor(t.TextMuted).SingleLine()
				matched, total := a.epgMatches(source, job)
				channels := 0
				if job.guide != nil {
					channels = len(job.guide.Programmes)
				}
				ui.Textf(c, tr("匹配 %d / %d 个频道 · EPG 内 %d 个频道"), matched, total, channels).FontSize(11).TextColor(t.TextMuted)
				stamp := tr("尚无成功更新")
				if !job.updated.IsZero() {
					stamp = tr("数据更新：") + job.updated.Local().Format("2006-01-02 15:04")
				}
				ui.Text(c, stamp).FontSize(11).TextColor(t.TextMuted)
				state, color := tr("等待更新"), t.TextMuted
				switch {
				case job.error != "":
					state, color = job.error, t.Warning
					if job.guide != nil {
						state += tr("；仍可使用已有数据")
					}
				case job.loading:
					state = tr("正在更新…")
				case job.guide != nil:
					state, color = tr("已就绪"), paletteFor(t).Success
				}
				ui.Row(c).Children(func() {
					ui.Text(c, state).FontSize(11).TextColor(color).Background(color.Alpha(.1)).Radius(6).Padding(4, 7)
				})
				ui.Spacer(c)
				ui.Row(c).Padding(8, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border.Alpha(.6)).Gap(6).AlignItems(ui.Center).Children(func() {
					text := tr("每 24 小时自动更新")
					if !job.loading && !job.nextTry.IsZero() {
						text = tr("下次：") + job.nextTry.Local().Format("01-02 15:04")
					}
					ui.Text(c, text).FontSize(10).TextColor(t.TextMuted).Grow(1).MinWidth(0)
					if settingsAction(c, tr("编辑 EPG ")+entry.ID, tr("编辑"), editIcon, false, false).Height(28).Padding(0, 8).Clicked() {
						a.editManualEPG(entry.ID)
					}
					if iconButton(c, tr("前移 EPG ")+entry.ID, leftIcon, false, index == 0).Clicked() {
						a.moveManualEPG(entry.ID, -1)
					}
					if iconButton(c, tr("后移 EPG ")+entry.ID, rightIcon, false, index == len(entries)-1).Clicked() {
						a.moveManualEPG(entry.ID, 1)
					}
				})
			})
		}
	})
}
