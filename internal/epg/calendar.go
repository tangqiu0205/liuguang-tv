package epg

import (
	"context"
	"sort"
	"time"
)

// Longer entries are almost always broken upstream timestamps, not TV programmes.
// This also bounds the number of day indexes one entry can produce (at most 4
// around unusual timezone transitions), independently of its original duration.
const MaxProgrammeDuration = 48 * time.Hour

func validDuration(p Programme) bool {
	return p.Stop.After(p.Start) && p.Stop.Sub(p.Start) <= MaxProgrammeDuration
}

// Retained never changes a published guide or retains its expired backing arrays.
func (g *Guide) Retained(now time.Time) *Guide {
	if g == nil {
		return nil
	}
	from := midnight(now).AddDate(0, 0, -7)
	until := midnight(now).AddDate(0, 0, 15)
	out := &Guide{Names: map[string]string{}, Programmes: map[string][]Programme{}}
	for id, ps := range g.Programmes {
		for _, p := range ps {
			if validDuration(p) && p.Stop.After(from) && p.Start.Before(until) {
				out.Programmes[id] = append(out.Programmes[id], p)
			}
		}
	}
	for name, id := range g.Names {
		if len(out.Programmes[id]) > 0 {
			out.Names[name] = id
		}
	}
	return out
}
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Days stores indexes into one immutable schedule, never programme copies.
// Programmes must be sorted by Start, as returned by Parse/Schedule.
type Calendar struct {
	Dates      []string
	Days       map[string][]int
	Programmes []Programme
}

func CalendarFor(ps []Programme, now time.Time) Calendar {
	cal, _ := CalendarForContext(context.Background(), ps, now)
	return cal
}

// CalendarForContext supports cancelling a build after a channel/source change.
// Day boundaries are computed once (with AddDate for DST), rather than formatting
// a date for every programme/day pair.
func CalendarForContext(ctx context.Context, ps []Programme, now time.Time) (Calendar, error) {
	cal := Calendar{Days: map[string][]int{}, Programmes: ps}
	from := midnight(now).AddDate(0, 0, -7)
	var starts []time.Time
	for i := 0; i <= 22; i++ {
		starts = append(starts, from.AddDate(0, 0, i))
	}
	keys := make([]string, 22)
	for i := range keys {
		keys[i] = starts[i].Format("2006-01-02")
	}
	for index, p := range ps {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return Calendar{}, err
			}
		}
		if !validDuration(p) || !p.Stop.After(from) || !p.Start.Before(starts[22]) {
			continue
		}
		first := sort.Search(22, func(i int) bool { return starts[i+1].After(p.Start) })
		for day := first; day < 22 && starts[day].Before(p.Stop); day++ {
			cal.Days[keys[day]] = append(cal.Days[keys[day]], index)
		}
	}
	if err := ctx.Err(); err != nil {
		return Calendar{}, err
	}
	for _, key := range keys {
		if len(cal.Days[key]) > 0 {
			cal.Dates = append(cal.Dates, key)
		}
	}
	return cal, nil
}
func (cal Calendar) At(date string, row int) Programme { return cal.Programmes[cal.Days[date][row]] }

// CurrentRow uses the sorted day indexes without materializing a day schedule.
func (cal Calendar) CurrentRow(date string, now time.Time) int {
	rows := cal.Days[date]
	split := sort.Search(len(rows), func(i int) bool { return cal.Programmes[rows[i]].Start.After(now) })
	for i := split - 1; i >= 0; i-- {
		if now.Before(cal.Programmes[rows[i]].Stop) {
			return i
		}
	}
	return -1
}
