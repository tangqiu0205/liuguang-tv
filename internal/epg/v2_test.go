package epg

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	_ "time/tzdata"
)

func TestCalendarCrossMidnightAndTimezone(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, zone)
	ps := []Programme{
		{Start: now.Add(-13 * time.Hour), Stop: now.Add(-11 * time.Hour), Title: "跨午夜"},
		{Start: now.Add(-14 * time.Hour), Stop: now.Add(-12 * time.Hour), Title: "午夜结束"},
	}
	cal := CalendarFor(ps, now)
	if len(cal.Dates) != 2 || len(cal.Days["2026-08-01"]) != 2 || len(cal.Days["2026-08-02"]) != 1 {
		t.Fatalf("unexpected calendar: %+v", cal)
	}
	utc := CalendarFor(ps, now.UTC())
	if len(utc.Dates) != 1 || utc.Dates[0] != "2026-08-01" {
		t.Fatalf("wrong viewer timezone: %+v", utc)
	}
}
func TestCalendarDSTAndExtremeDuration(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, loc)
	start := time.Date(2026, 3, 7, 23, 30, 0, 0, loc)
	stop := time.Date(2026, 3, 9, 0, 30, 0, 0, loc)
	cal := CalendarFor([]Programme{{Start: start, Stop: stop}}, now)
	if len(cal.Dates) != 3 {
		t.Fatalf("DST lost or duplicated a day: %v", cal.Dates)
	}
	cal = CalendarFor([]Programme{{Start: now.AddDate(-100, 0, 0), Stop: now.AddDate(100, 0, 0)}}, now)
	if len(cal.Dates) != 0 {
		t.Fatalf("extreme duration was not rejected: %d", len(cal.Dates))
	}
}
func TestRetainedIsImmutable(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	old := Programme{Start: now.AddDate(0, 0, -30), Stop: now.AddDate(0, 0, -29)}
	current := Programme{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour)}
	future := Programme{Start: now.AddDate(0, 0, 30), Stop: now.AddDate(0, 0, 31)}
	g := &Guide{Programmes: map[string][]Programme{"one": {old, current, future}, "expired": {old}}, Names: map[string]string{"demo": "one", "old": "expired"}}
	kept := g.Retained(now)
	if len(kept.Programmes) != 1 || len(kept.Schedule("one", "")) != 1 || len(kept.Names) != 1 {
		t.Fatalf("bad retained guide: %+v", kept)
	}
	kept.Programmes["one"][0].Title = "changed"
	if len(g.Programmes["one"]) != 3 || g.Programmes["one"][1].Title != "" {
		t.Fatal("pruning mutated a published guide")
	}
}
func TestRejectExtraRootAndNestedProgramme(t *testing.T) {
	for _, raw := range []string{sample + "<tv/>", sample + "junk", `<tv><unknown><programme channel="one" start="20260801190000" stop="20260801200000"/></unknown></tv>`} {
		if _, err := Parse(strings.NewReader(raw)); err == nil {
			t.Fatal("accepted malformed guide")
		}
	}
}
func TestLoadRetainedDoesNotReplaceUsefulCacheWithExpiredData(t *testing.T) {
	now := time.Date(2026, 8, 1, 11, 15, 0, 0, time.UTC)
	var raw atomic.Value
	raw.Store(sample)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw.Load().(string)) }))
	defer server.Close()
	dir := t.TempDir()
	if _, err := LoadRetained(context.Background(), server.URL, dir, now); err != nil {
		t.Fatal(err)
	}
	raw.Store(strings.ReplaceAll(sample, "20260801", "20000101"))
	if _, err := LoadRetained(context.Background(), server.URL, dir, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("wanted expired error: %v", err)
	}
	g, at, err := CachedRetained(dir, server.URL, now)
	if err != nil || at.IsZero() || len(g.Schedule("cctv1", "")) != 2 {
		t.Fatalf("lost valid cache: %v", err)
	}
	if _, _, err := CachedRetained(dir, server.URL, now.AddDate(0, 1, 0)); !errors.Is(err, ErrExpired) {
		t.Fatal("old cache did not expire")
	}
}
func TestCleanupOnlyRemovesOwnedExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	files := map[string]time.Duration{
		filepath.Base(cachePath(dir, "active")):  10 * 24 * time.Hour,
		filepath.Base(cachePath(dir, "ancient")): 31 * 24 * time.Hour,
		filepath.Base(cachePath(dir, "removed")): 8 * 24 * time.Hour,
		filepath.Base(cachePath(dir, "recent")):  time.Hour,
		"epg-123.tmp":                            25 * time.Hour,
		"epg-456.tmp":                            time.Hour,
		"unrelated.xml":                          100 * 24 * time.Hour,
	}
	for file, age := range files {
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, []byte("cache"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	count, err := Cleanup(context.Background(), dir, []string{"active", "ancient"}, now)
	if err != nil || count != 3 {
		t.Fatalf("bad cleanup: %d %v", count, err)
	}
	for _, file := range []string{filepath.Base(cachePath(dir, "active")), filepath.Base(cachePath(dir, "recent")), "epg-456.tmp", "unrelated.xml"} {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			t.Fatalf("removed protected file %s", file)
		}
	}
}
func TestFailuresAreActionableAndRedacted(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{HTTPError(401), "HTTP 401"}, {ErrGzip, "gzip"}, {ErrCache, "磁盘"}, {ErrXML, "XMLTV"}, {context.DeadlineExceeded, "超时"},
		{fmt.Errorf("https://secret.example/epg?token=secret"), "读取失败"},
	} {
		got := FailureReason(tc.err)
		if !strings.Contains(got, tc.want) || strings.Contains(got, "secret") {
			t.Fatalf("bad failure: %q", got)
		}
	}
}
func TestCancelledDownloadAndCorruptGzip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := LoadRetained(ctx, server.URL, t.TempDir(), time.Now()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bad cancellation: %v", err)
	}
	server.Close()
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "bad.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	fmt.Fprint(gz, sample)
	gz.Close()
	f.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-8] ^= 0xff // corrupt CRC, even though the XML itself is complete
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(context.Background(), path, dir); !errors.Is(err, ErrGzip) {
		t.Fatalf("accepted corrupt gzip: %v", err)
	}
}

func TestGzipExpansionIsBoundedAndLeavesNoStagingFiles(t *testing.T) {
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	fmt.Fprint(gz, sample)
	padding := strings.Repeat(" ", 1<<20)
	for i := 0; i < 64; i++ {
		fmt.Fprint(gz, padding)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "large.xml.gz")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Load(context.Background(), path, dir); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expansion not bounded: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed download left cache or temporary files: %v %v", entries, err)
	}
}

func TestTimeOffsetsAndOverlappingProgrammes(t *testing.T) {
	for _, input := range []string{"20260801190000 +0800", "20260801110000 +0000", "20260801063000 -0430", "20260801110000"} {
		got, err := parseTime(input)
		want := time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)
		if err != nil || !got.Equal(want) {
			t.Fatalf("wrong offset for %q: %v %v", input, got, err)
		}
	}
	now := time.Now()
	ps := []Programme{
		{Start: now.Add(-2 * time.Hour), Stop: now.Add(time.Hour), Title: "long"},
		{Start: now.Add(-time.Hour), Stop: now.Add(-time.Minute), Title: "ended overlap"},
		{Start: now.Add(time.Hour), Stop: now.Add(2 * time.Hour), Title: "next"},
	}
	current, next := NowNext(ps, now)
	if current == nil || current.Title != "long" || next == nil || next.Title != "next" {
		t.Fatal("overlap lookup lost active programme")
	}
	if current, _ := NowNext(ps, now.Add(3*time.Hour)); current != nil {
		t.Fatal("ended programme still active")
	}
}

func BenchmarkLargeCalendar(b *testing.B) {
	now := time.Now()
	var ps []Programme
	for i := 0; i < 10000; i++ {
		start := now.Add(time.Duration(i) * time.Minute)
		ps = append(ps, Programme{Start: start, Stop: start.Add(time.Minute)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalendarFor(ps, now)
	}
}
