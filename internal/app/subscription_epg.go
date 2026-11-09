package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

type subscriptionEPGStats struct {
	matched, total, channels int
	revision, catalogVersion uint64
	updated                  time.Time
	guides                   []*epg.Guide
}

func (a *application) subscriptionEPGSummary(sub store.Subscription) (string, string) {
	if len(sub.EPGURLs) == 0 {
		return tr("未提供 EPG"), tr("订阅未提供节目单地址，可在 EPG 页面设置备用来源。")
	}
	var guides []*epg.Guide
	var details []string
	for _, source := range sub.EPGURLs {
		job := a.epg.jobs[source]
		state := tr("等待加载")
		if job != nil {
			if job.guide != nil && !slices.Contains(guides, job.guide) {
				guides = append(guides, job.guide)
			}
			switch {
			case job.error != "":
				state = job.error
				if job.guide != nil {
					state += tr("；仍可使用已有数据")
				}
			case job.loading:
				state = tr("正在更新…")
			case job.guide != nil:
				state = tr("已就绪")
			}
		}
		details = append(details, epgSourceDisplay(source)+"："+state)
	}
	if a.epg.subscriptionStats == nil {
		a.epg.subscriptionStats = make(map[string]subscriptionEPGStats)
	}
	stats, exists := a.epg.subscriptionStats[sub.ID]
	if !exists || stats.revision != sub.Revision || stats.catalogVersion != a.epg.catalogVersion || stats.updated != sub.UpdatedAt || !slices.Equal(stats.guides, guides) {
		stats = subscriptionEPGStats{revision: sub.Revision, catalogVersion: a.epg.catalogVersion, updated: sub.UpdatedAt, guides: guides}
		seenChannels := map[string]bool{}
		for _, ch := range sub.Channels {
			key := playlist.ChannelKey(ch)
			if seenChannels[key] {
				continue
			}
			seenChannels[key] = true
			stats.total++
			for _, guide := range guides {
				if len(guide.Schedule(ch.TVGID, ch.Name)) > 0 {
					stats.matched++
					break
				}
			}
		}
		seenIDs := map[string]bool{}
		for _, guide := range guides {
			for id := range guide.Programmes {
				seenIDs[id] = true
			}
		}
		stats.channels = len(seenIDs)
		a.epg.subscriptionStats[sub.ID] = stats
	}
	summary := fmt.Sprintf(tr("匹配 %d / %d 个频道 · EPG 内 %d 个频道"), stats.matched, stats.total, stats.channels)
	if len(guides) == 0 {
		summary = tr("EPG 尚无可用数据")
	}
	return summary, strings.Join(details, "\n")
}
