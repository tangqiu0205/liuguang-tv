package epg

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const sample = `<tv>
<channel id="cctv1"><display-name>CCTV-1</display-name></channel>
<programme channel="cctv1" start="20260801193000 +0800" stop="20260801200000 +0800"><title>天气预报</title></programme>
<programme channel="cctv1" start="20260801190000 +0800" stop="20260801193000 +0800"><title>新闻联播</title><desc>每日新闻</desc></programme>
</tv>`

func TestParseMatchAndNowNext(t *testing.T) {
	g, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range [][2]string{{"CCTV1", "ignored"}, {"", "cctv 1"}} {
		ps := g.Schedule(query[0], query[1])
		now := time.Date(2026, 8, 1, 11, 15, 0, 0, time.UTC)
		current, next := NowNext(ps, now)
		if current == nil || current.Title != "新闻联播" || next == nil || next.Title != "天气预报" {
			t.Fatalf("bad current/next: %v %v", current, next)
		}
		current, _ = NowNext(ps, now.Add(15*time.Minute))
		if current == nil || current.Title != "天气预报" {
			t.Fatal("stop must be exclusive")
		}
	}
}
func TestAmbiguousNamesDoNotMatch(t *testing.T) {
	raw := strings.Replace(sample, "</tv>", `<channel id="other"><display-name>CCTV 1</display-name></channel></tv>`, 1)
	g, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Schedule("", "CCTV-1")) != 0 {
		t.Fatal("ambiguous name matched")
	}
	if len(g.Schedule("cctv1", "CCTV-1")) != 2 {
		t.Fatal("ID must win")
	}
}
func TestRejectInvalid(t *testing.T) {
	for _, raw := range []string{"", "<html></html>", "<tv>", "<tv></tv>", `<tv><programme channel="a" start="bad" stop="bad"/></tv>`} {
		if _, err := Parse(strings.NewReader(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := Parse(strings.NewReader(sample + strings.Repeat(" ", MaxBytes))); err == nil {
		t.Fatal("accepted oversized input")
	}
}
func TestLoadGzipAndRetainCache(t *testing.T) {
	var broken atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broken.Load() {
			fmt.Fprint(w, "bad XML")
			return
		}
		gz := gzip.NewWriter(w)
		fmt.Fprint(gz, sample)
		gz.Close()
	}))
	defer server.Close()
	dir := t.TempDir()
	if _, err := Load(context.Background(), server.URL, dir); err != nil {
		t.Fatal(err)
	}
	broken.Store(true)
	if _, err := Load(context.Background(), server.URL, dir); err == nil {
		t.Fatal("accepted invalid update")
	}
	cached, err := Cached(dir, server.URL)
	if err != nil || len(cached.Schedule("cctv1", "")) != 2 {
		t.Fatalf("lost cache: %v", err)
	}
}
func TestCancelledLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Load(ctx, "https://example.invalid/epg.xml", t.TempDir()); err == nil {
		t.Fatal("ignored cancellation")
	}
}
