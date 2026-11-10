package app

import (
	"context"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/loadqueue"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

type epgJob struct {
	cancel                      context.CancelFunc
	guide                       *epg.Guide
	loading                     bool
	updated, attempted, nextTry time.Time
	error                       string
	matched, total              int
	matchVersion                uint64
	matchedGuide                *epg.Guide
}
type epgState struct {
	slots                          chan struct{}
	jobs                           map[string]*epgJob
	subscriptionStats              map[string]subscriptionEPGStats
	byURL                          map[string][]string
	bySource                       map[string][]string
	calendarCancel                 context.CancelFunc
	calendarRequest                uint64
	calendarLoading                bool
	candidates                     map[string][]playlist.Channel
	catalogVersion                 uint64
	show                           bool
	list                           ui.ListState
	calendar                       epg.Calendar
	calendarFirst                  *epg.Programme
	calendarCount                  int
	calendarChannel, calendarToday string
	date, listKey                  string
	calendarVersion                uint64
	nextCheck                      time.Time
	cleanupBusy                    bool
	cancelCleanup                  context.CancelFunc
	nextCleanup                    time.Time
	cleanupMessage                 string
}

func (a *application) syncEPG(force bool) {
	if a.window == nil && a.dispatch == nil {
		return
	}
	if a.epg.jobs == nil {
		a.epg.jobs = map[string]*epgJob{}
	}
	if a.epg.slots == nil {
		a.epg.slots = make(chan struct{}, 3)
	}
	wanted := map[string]bool{}
	a.epg.byURL = map[string][]string{}
	a.epg.bySource = map[string][]string{}
	a.epg.candidates = map[string][]playlist.Channel{}
	for _, sub := range a.state.Subscriptions {
		if sub.Disabled {
			continue
		}
		a.epg.bySource[sub.ID] = sub.EPGURLs
		for _, source := range sub.EPGURLs {
			wanted[source] = true
			a.epg.candidates[source] = append(a.epg.candidates[source], sub.Channels...)
		}
		for _, ch := range sub.Channels {
			for _, line := range playlist.Lines(ch) {
				a.epg.byURL[line.URL] = append(a.epg.byURL[line.URL], sub.EPGURLs...)
			}
		}
	}
	for _, entry := range a.state.ManualEPGSources() {
		wanted[entry.Source] = true
		if len(a.state.Subscriptions) == 0 {
			a.epg.candidates[entry.Source] = a.state.Channels
		} else {
			for _, sub := range a.state.Subscriptions {
				if !sub.Disabled {
					a.epg.candidates[entry.Source] = append(a.epg.candidates[entry.Source], sub.Channels...)
				}
			}
		}
	}
	a.epg.catalogVersion++
	a.epg.subscriptionStats = nil
	for source, job := range a.epg.jobs {
		if !wanted[source] {
			if job.cancel != nil {
				job.cancel()
			}
			delete(a.epg.jobs, source)
		}
	}
	for source := range wanted {
		job := a.epg.jobs[source]
		if job != nil && (job.loading || (!force && time.Now().Before(job.nextTry))) {
			continue
		}
		a.startEPG(source, force)
	}
	a.epg.nextCheck = time.Now().Add(time.Minute)
	if !time.Now().Before(a.epg.nextCleanup) {
		a.cleanupEPG()
	}
}

// Poll only source jobs: do not rebuild a large channel-to-source index on every tick.
func (a *application) pollEPG() {
	for source, job := range a.epg.jobs {
		if !job.loading && !time.Now().Before(job.nextTry) {
			a.startEPG(source, false)
		}
	}
	a.epg.nextCheck = time.Now().Add(time.Minute)
	if !time.Now().Before(a.epg.nextCleanup) {
		a.cleanupEPG()
	}
}

func (a *application) loadEPG(source string) { a.startEPG(source, true) }

func (a *application) startEPG(source string, force bool) {
	if old := a.epg.jobs[source]; old != nil && old.loading {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &epgJob{cancel: cancel, loading: true, attempted: time.Now()}
	if old := a.epg.jobs[source]; old != nil {
		job.guide, job.updated, job.error = old.guide, old.updated, old.error
		if old.cancel != nil {
			old.cancel()
		}
	}
	a.epg.jobs[source] = job
	needCache := job.guide == nil
	currentGuide, currentAt := job.guide, job.updated
	lastCheck := a.state.EPGChecks[source]
	dir := filepath.Join(filepath.Dir(a.path), "epg")
	slots := a.epg.slots
	go func() {
		defer cancel()
		request, release, err := loadqueue.Acquire(ctx, slots, time.Minute)
		if err != nil {
			a.update(func() { a.completeEPG(source, job, nil, err, time.Now()) })
			return
		}
		defer release()
		if needCache {
			if cached, at, err := epg.CachedRetained(dir, source, time.Now()); err == nil {
				currentGuide, currentAt = cached, at
				a.update(func() {
					if a.epg.jobs[source] == job {
						job.guide, job.updated = cached, at
					}
				})
			}
		}
		if currentAt.After(lastCheck) {
			lastCheck = currentAt
		}
		if !force && !dailyRefreshDue(lastCheck, time.Now()) {
			a.update(func() {
				if a.epg.jobs[source] != job {
					return
				}
				job.guide, job.updated = currentGuide, currentAt
				job.loading = false
				job.attempted = lastCheck
				job.nextTry = lastCheck.Add(dailyRefreshInterval)
				if currentGuide == nil {
					job.error = tr("暂无可用缓存，可手动刷新")
				}
			})
			return
		}
		guide, err := epg.LoadRetained(request, source, dir, time.Now())
		a.update(func() { a.completeEPG(source, job, guide, err, time.Now()) })
	}()
}
func (a *application) completeEPG(source string, job *epgJob, guide *epg.Guide, err error, now time.Time) {
	if a.epg.jobs[source] != job {
		return
	}
	job.loading = false
	job.nextTry = now.Add(dailyRefreshInterval)
	job.error = ""
	if guide != nil {
		job.guide, job.updated = guide, now
		job.matchedGuide = nil // Do not retain a previous guide through the stats cache.
	}
	if err != nil {
		job.error = epg.FailureReason(err)
	}
	if a.state.EPGChecks == nil {
		a.state.EPGChecks = map[string]time.Time{}
	}
	a.state.EPGChecks[source] = now
	if a.path != "" {
		a.save()
	}
}

func (a *application) epgSources(ch playlist.Channel) []string {
	seen := map[string]bool{}
	var sources []string
	appendSource := func(urls []string) {
		for _, source := range urls {
			if !seen[source] {
				sources = append(sources, source)
				seen[source] = true
			}
		}
	}
	if ch.SourceID != "" {
		appendSource(a.epg.bySource[ch.SourceID])
	} else {
		for _, line := range playlist.Lines(ch) {
			if line.SourceID != "" {
				appendSource(a.epg.bySource[line.SourceID])
			} else {
				appendSource(a.epg.byURL[line.URL])
			}
		}
	}
	for _, entry := range a.state.ManualEPGSources() {
		appendSource([]string{entry.Source})
	}
	return sources
}
func (a *application) schedule(ch playlist.Channel) []epg.Programme {
	return a.scheduleAt(ch, time.Now())
}
func (a *application) scheduleAt(ch playlist.Channel, now time.Time) []epg.Programme {
	var historical []epg.Programme
	for _, source := range a.epgSources(ch) {
		job := a.epg.jobs[source]
		if job == nil {
			continue
		}
		ps := job.guide.Schedule(ch.TVGID, ch.Name)
		if len(ps) == 0 {
			continue
		}
		current, next := epg.NowNext(ps, now)
		if current != nil || next != nil {
			return ps
		}
		if historical == nil {
			historical = ps
		}
	}
	return historical
}
func (a *application) epgMatches(source string, job *epgJob) (int, int) {
	if job.matchedGuide == job.guide && job.matchVersion == a.epg.catalogVersion {
		return job.matched, job.total
	}
	seen := map[string]bool{}
	matched, total := 0, 0
	for _, ch := range a.epg.candidates[source] {
		key := playlist.ChannelKey(ch)
		if seen[key] {
			continue
		}
		seen[key] = true
		total++
		if len(job.guide.Schedule(ch.TVGID, ch.Name)) > 0 {
			matched++
		}
	}
	job.matched, job.total, job.matchVersion, job.matchedGuide = matched, total, a.epg.catalogVersion, job.guide
	return matched, total
}
func (a *application) cleanupEPG() {
	if a.epg.cleanupBusy || (a.window == nil && a.dispatch == nil) {
		return
	}
	a.epg.cleanupBusy = true
	a.epg.cleanupMessage = tr("正在清理过期缓存…")
	a.epg.nextCleanup = time.Now().Add(24 * time.Hour)
	sources := make([]string, 0, len(a.epg.jobs))
	for source := range a.epg.jobs {
		sources = append(sources, source)
	}
	dir := filepath.Join(filepath.Dir(a.path), "epg")
	ctx, cancel := context.WithCancel(context.Background())
	a.epg.cancelCleanup = cancel
	generation := a.epgGeneration
	go func() {
		defer cancel()
		count, err := epg.Cleanup(ctx, dir, sources, time.Now())
		a.update(func() {
			if generation != a.epgGeneration {
				return
			}
			a.epg.cleanupBusy = false
			a.epg.cancelCleanup = nil
			a.epg.cleanupMessage = cleanupResult(count, err)
		})
	}()
}
func (a *application) epgSummary(c *ui.Context) {
	t := c.Theme()
	current, next := epg.NowNext(a.schedule(a.playingEPGChannel()), time.Now())
	ui.Row(c).Label(tr("节目摘要")).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Column(c).Label(tr("播放频道台标")).Size(32, 32).Shrink(0).Center().Radius(9).Background(selectedBackground(t)).Children(func() {
			if logo := a.logos.get(a.activeChannel.Logo, c.Services()); logo != nil {
				ui.Image(c, logo).Size(30, 30).Shrink(0)
			} else {
				ui.Icon(c, tvIcon).Size(16, 16).TextColor(t.Accent)
			}
		})
		ui.Column(c).Grow(0.3).MinWidth(32).Children(func() {
			ui.Text(c, a.now).Label(tr("播放频道名")).FontSize(16).Bold().SingleLine().Tooltip(a.now)
		})
		ui.Column(c).Label(tr("频道节目分隔线")).Width(2).Height(30).Radius(1).AlignSelf(ui.Center).Shrink(0).Background(t.TextMuted.Alpha(.4))
		ui.Column(c).Grow(1).MinWidth(32).Gap(5).Children(func() {
			title := tr("暂无节目信息")
			if current != nil {
				title = current.Start.Local().Format("15:04") + "–" + current.Stop.Local().Format("15:04") + "  " + current.Title
			}
			ui.Text(c, title).Label(tr("当前节目")).FontSize(12).Bold().SingleLine().Tooltip(title)
			text := tr("节目单仅供参考，不影响直播播放")
			if next != nil {
				text = tr("接下来：") + next.Start.Local().Format("15:04") + "  " + next.Title
			}
			ui.Text(c, text).Label(tr("下一个节目")).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(text)
		})
		if ui.Button(c, tr("节目单")).Height(30).Shrink(0).Clicked() {
			if a.epg.show {
				a.closeEPGPanel()
			} else {
				a.epg.show = true
				a.settle = 3
				a.epg.calendarChannel = ""
				a.epg.listKey = ""
			}
		}
	})
}
