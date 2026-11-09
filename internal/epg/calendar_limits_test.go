package epg

import (
	"context"
	"testing"
	"time"
)

func TestCalendarUsesIndexesAndRejectsAbnormalDurations(t *testing.T) {
	now := time.Now()
	ps := make([]Programme, 10000)
	for i := range ps {
		ps[i] = Programme{Start: now.AddDate(0, 0, -30), Stop: now.AddDate(0, 0, 30), Title: "bad upstream interval"}
	}
	cal := CalendarFor(ps, now)
	if len(cal.Dates) != 0 || len(cal.Days) != 0 {
		t.Fatal("abnormal intervals amplified calendar data")
	}
	// Valid cross-day entries are represented only by indexes into the original schedule.
	day := midnight(now)
	for i := range ps {
		ps[i] = Programme{Start: day.Add(-time.Hour), Stop: day.Add(time.Hour), Title: "cross midnight"}
	}
	cal = CalendarFor(ps, now)
	if len(cal.Dates) != 2 || len(cal.Days[day.Format("2006-01-02")]) != len(ps) {
		t.Fatal("lost valid cross-midnight rows")
	}
	if &cal.Programmes[0] != &ps[0] {
		t.Fatal("calendar copied the programme schedule")
	}
	if cal.At(day.Format("2006-01-02"), 0).Title != "cross midnight" {
		t.Fatal("bad index lookup")
	}
}
func TestRetentionRejectsLongIntervalsWithoutChangingInput(t *testing.T) {
	now := time.Now()
	g := &Guide{Programmes: map[string][]Programme{"one": {
		{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour), Title: "valid"},
		{Start: now.Add(-time.Hour), Stop: now.Add(72 * time.Hour), Title: "invalid"},
	}}}
	retained := g.Retained(now)
	if len(retained.Programmes["one"]) != 1 || len(g.Programmes["one"]) != 2 {
		t.Fatal("retention did not safely filter abnormal durations")
	}
}
func TestCancelledCalendarReturnsNoPartialData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cal, err := CalendarForContext(ctx, []Programme{{Start: time.Now(), Stop: time.Now().Add(time.Hour)}}, time.Now())
	if err != context.Canceled || len(cal.Dates) != 0 {
		t.Fatal("cancelled build returned partial data")
	}
}
