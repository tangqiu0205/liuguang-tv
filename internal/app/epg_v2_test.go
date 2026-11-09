package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func v2Guide(now time.Time, rows int) *epg.Guide {
	y, m, d := now.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	g := &epg.Guide{Names: map[string]string{"demo": "one"}, Programmes: map[string][]epg.Programme{}}
	for offset := -1; offset <= 1; offset++ {
		start := day.AddDate(0, 0, offset)
		next := start.AddDate(0, 0, 1)
		step := next.Sub(start) / time.Duration(rows)
		for i := 0; i < rows; i++ {
			at := start.Add(time.Duration(i) * step)
			g.Programmes["one"] = append(g.Programmes["one"], epg.Programme{Start: at, Stop: at.Add(step), Title: fmt.Sprintf("%s 节目 %04d", start.Format("01-02"), i)})
		}
	}
	return g
}
func v2CalendarApp(now time.Time, rows int) *application {
	a := largeChannelApp()
	ch := a.state.Channels[0]
	ch.Name, ch.TVGID = "Demo", "one"
	a.state.Channels = []playlist.Channel{ch}
	a.activeChannel, a.now = ch, ch.Name
	a.epg.jobs = map[string]*epgJob{"guide": {guide: v2Guide(now, rows), nextTry: now.Add(time.Hour)}}
	a.epg.byURL = map[string][]string{ch.URL: {"guide"}}
	a.epg.nextCheck = now.Add(time.Hour)
	a.epg.show = true
	return a
}
func TestEPGMultiDateAndVirtualization(t *testing.T) {
	now := time.Now()
	a := v2CalendarApp(now, 1000)
	tt := ui.NewTester(a.view, 1280, 820)
	if a.epg.date != now.Format("2006-01-02") || len(a.epg.calendar.Dates) != 3 {
		t.Fatalf("bad default calendar: %s %v", a.epg.date, a.epg.calendar.Dates)
	}
	first, last := a.epg.list.Visible()
	if last-first > 40 {
		t.Fatalf("EPG list is not virtualized: %d..%d", first, last)
	}
	row := a.epg.calendar.CurrentRow(a.epg.date, now)
	if row < 0 || !tt.HasText(a.epg.calendar.At(a.epg.date, row).Title) {
		t.Fatal("current programme was not brought into view")
	}
	panel := requiredRect(t, tt, "EPG 节目单")
	contained(t, "关闭节目单", panel, requiredRect(t, tt, "关闭节目单"))
	if err := tt.Click("下一节目日期"); err != nil {
		t.Fatal(err)
	}
	if a.epg.date != now.AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatal("next day did not switch")
	}
	if err := tt.Click("上一节目日期"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("上一节目日期"); err != nil {
		t.Fatal(err)
	}
	if a.epg.date != now.AddDate(0, 0, -1).Format("2006-01-02") {
		t.Fatal("previous day did not switch")
	}
	list := requiredRect(t, tt, "节目列表")
	tt.Wheel(list.X+list.W/2, list.Y+list.H/2, 0, 800)
	tt.Advance(time.Second)
	first, last = a.epg.list.Visible()
	if first == 0 || last-first > 40 {
		t.Fatalf("bad virtualized scroll %d..%d", first, last)
	}
	if err := tt.Click("节目日期"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(epgDateLabel(now.AddDate(0, 0, 1).Format("2006-01-02"), now)); err != nil {
		t.Fatal(err)
	}
	if a.epg.date != now.AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatal("date menu did not switch")
	}
	tt.SetSize(1000, 620)
	tt.SetScale(1.5)
	panel = requiredRect(t, tt, "EPG 节目单")
	contained(t, "日期导航", panel, requiredRect(t, tt, "节目日期导航"))
	contained(t, "关闭节目单", panel, requiredRect(t, tt, "关闭节目单"))
}
func TestEPGCalendarRefreshAndChannelSwitch(t *testing.T) {
	now := time.Now()
	a := v2CalendarApp(now, 3)
	a.prepareEPGCalendar(now)
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	a.selectEPGDate(tomorrow)
	a.epg.jobs["guide"].guide = v2Guide(now, 4)
	a.prepareEPGCalendar(now)
	if a.epg.date != tomorrow || len(a.epg.calendar.Days[tomorrow]) != 4 {
		t.Fatal("refresh lost selected date or kept stale rows")
	}
	a.activeChannel.Key = "another-channel"
	a.prepareEPGCalendar(now)
	if a.epg.date != now.Format("2006-01-02") {
		t.Fatal("channel switch did not reset to today")
	}
	a.selectEPGDate("1900-01-01")
	if a.epg.date == "1900-01-01" {
		t.Fatal("selected a day without data")
	}
	a.prepareEPGCalendar(now.AddDate(0, 0, 1))
	if a.epg.date != tomorrow {
		t.Fatal("midnight did not advance default date")
	}
}
func TestCompleteEPGFailureAndStaleResult(t *testing.T) {
	now := time.Now()
	g := v2Guide(now, 3)
	job := &epgJob{guide: g, updated: now.Add(-time.Hour), loading: true}
	a := &application{}
	a.epg.jobs = map[string]*epgJob{"source": job}
	oldTime := job.updated
	a.completeEPG("source", job, nil, epg.HTTPError(403), now)
	if job.guide != g || job.updated != oldTime || job.nextTry != now.Add(24*time.Hour) || !strings.Contains(job.error, "403") {
		t.Fatal("failed refresh lost cache metadata")
	}
	replacement := &epgJob{}
	a.epg.jobs["source"] = replacement
	a.completeEPG("source", job, v2Guide(now, 5), nil, now)
	if replacement.guide != nil || job.guide != g {
		t.Fatal("stale result was applied")
	}
	a.completeEPG("source", replacement, g, nil, now)
	if replacement.guide != g || replacement.updated != now || replacement.nextTry != now.Add(24*time.Hour) || replacement.error != "" {
		t.Fatal("bad successful update")
	}
}
func TestScheduleUsesLiveFallbackInsteadOfExpiredPrimary(t *testing.T) {
	now := time.Now()
	a := v2CalendarApp(now, 3)
	primary := &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{Start: now.Add(-48 * time.Hour), Stop: now.Add(-47 * time.Hour), Title: "过期"}}}}
	a.epg.jobs["guide"].guide = primary
	a.state.EPGSource = "fallback"
	a.epg.jobs["fallback"] = &epgJob{guide: v2Guide(now, 4)}
	current, _ := epg.NowNext(a.scheduleAt(a.activeChannel, now), now)
	if current == nil || current.Title == "过期" {
		t.Fatal("expired primary hid current fallback")
	}
}
func TestEPGSourceStatusAndSingleRefresh(t *testing.T) {
	now := time.Now().UTC()
	var calls1, calls2 atomic.Int32
	handler := func(counter *atomic.Int32) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			fmt.Fprintf(w, `<tv><programme channel="one" start="%s +0000" stop="%s +0000"><title>直播节目</title></programme></tv>`, now.Add(-time.Hour).Format("20060102150405"), now.Add(time.Hour).Format("20060102150405"))
		}
	}
	s1, s2 := httptest.NewServer(handler(&calls1)), httptest.NewServer(handler(&calls2))
	defer s1.Close()
	defer s2.Close()
	source1, source2 := s1.URL+"/?token=private-token", s2.URL
	ch := playlist.Channel{Name: "Demo", TVGID: "one", URL: "https://example.com/live"}
	queue := make(chan func(), 64)
	a := &application{path: filepath.Join(t.TempDir(), "state.json"), state: store.State{EPGSource: source1, Subscriptions: []store.Subscription{
		{ID: "one", Name: "来源一", EPGURLs: []string{source1}, Channels: []playlist.Channel{ch, ch}},
		{ID: "two", Name: "来源二", EPGURLs: []string{source2}, Channels: []playlist.Channel{ch}},
	}}}
	a.dispatch = func(fn func()) { queue <- fn }
	t.Cleanup(func() { a.cleanup() })
	a.syncEPG(false)
	pumpPlayback(t, queue, func() bool {
		return len(a.epg.jobs) == 2 && !a.epg.jobs[source1].loading && !a.epg.jobs[source2].loading && !a.epg.cleanupBusy
	})
	for source, job := range a.epg.jobs {
		matched, total := a.epgMatches(source, job)
		if matched != 1 || total != 1 || job.updated.IsZero() || job.error != "" {
			t.Fatalf("bad source status: %+v (%d/%d)", job, matched, total)
		}
	}
	tt := ui.NewTester(func(c *ui.Context) { ui.Column(c).Fill().Children(func() { a.epgSettings(c, 1000) }) }, 1000, 1200)
	if tt.HasText("private-token") {
		t.Fatal("token leaked into status UI")
	}
	if err := tt.Click("刷新 EPG 源 " + epgSourceID(source1)); err != nil {
		t.Fatal(err)
	}
	pumpPlayback(t, queue, func() bool { return !a.epg.jobs[source1].loading })
	if calls1.Load() != 2 || calls2.Load() != 1 {
		t.Fatalf("single refresh updated other sources: %d %d", calls1.Load(), calls2.Load())
	}
}
func TestDisableSourceCancelsAndRejectsLateResults(t *testing.T) {
	a := &application{path: filepath.Join(t.TempDir(), "state.json"), state: store.State{Subscriptions: []store.Subscription{{ID: "one", Disabled: true, EPGURLs: []string{"removed"}}}}}
	queue := make(chan func(), 16)
	a.dispatch = func(fn func()) { queue <- fn }
	cancelled := false
	old := &epgJob{cancel: func() { cancelled = true }, loading: true}
	a.epg.jobs = map[string]*epgJob{"removed": old}
	t.Cleanup(a.cleanup)
	a.syncEPG(false)
	if !cancelled || len(a.epg.jobs) != 0 {
		t.Fatal("disabled EPG source stayed active")
	}
	a.completeEPG("removed", old, v2Guide(time.Now(), 3), nil, time.Now())
	if len(a.epg.jobs) != 0 {
		t.Fatal("late result revived removed source")
	}
	pumpPlayback(t, queue, func() bool { return !a.epg.cleanupBusy })
}
func TestEPGOperationsDoNotRestartPlayback(t *testing.T) {
	a, engine, _, ch := playbackFixture(t)
	a.play(ch)
	now := time.Now()
	generation := a.generation
	a.epg.jobs = map[string]*epgJob{"guide": {guide: v2Guide(now, 3)}}
	a.epg.byURL = map[string][]string{ch.URL: {"guide"}}
	a.completeEPG("guide", a.epg.jobs["guide"], nil, errors.New("network failed"), now)
	a.prepareEPGCalendar(now)
	if len(a.epg.calendar.Dates) > 0 {
		a.selectEPGDate(a.epg.calendar.Dates[0])
	}
	a.toggleWindowCinema()
	a.toggleCinema()
	a.toggleCinema()
	if len(engine.streams) != 1 || a.generation != generation || a.activeChannel.URL != ch.URL {
		t.Fatal("EPG disturbed current stream")
	}
	select {
	case <-engine.streams[0].done:
		t.Fatal("EPG stopped playback")
	default:
	}
}
func BenchmarkEPGVirtualizedPanel(b *testing.B) {
	for _, rows := range []int{100, 10000} {
		b.Run(fmt.Sprint(rows), func(b *testing.B) {
			a := v2CalendarApp(time.Now(), rows)
			a.prepareEPGCalendar(time.Now())
			// Force real redraws, not idle timer advances.
			tt := ui.NewTester(a.view, 1280, 820)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tt.Frame()
			}
		})
	}
}
func TestCalendarDatesAreSorted(t *testing.T) {
	a := v2CalendarApp(time.Now(), 2)
	a.prepareEPGCalendar(time.Now())
	if !sort.StringsAreSorted(a.epg.calendar.Dates) {
		t.Fatal("dates out of order")
	}
}
