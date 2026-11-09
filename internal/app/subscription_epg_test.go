package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestSubscriptionEPGSummaryIsScopedAndDeduplicated(t *testing.T) {
	one := playlist.Channel{Name: "One", TVGID: "one", URL: "https://example.org/one"}
	missing := playlist.Channel{Name: "Missing", TVGID: "missing", URL: "https://example.org/missing"}
	guide := &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{}}, "two": {{}}}}
	source := "https://example.org/guide.xml?token=private"
	sub := store.Subscription{ID: "one", Channels: []playlist.Channel{one, one, missing}, EPGURLs: []string{source, source}}
	a := &application{state: store.State{EPGSource: "fallback"}}
	a.epg.jobs = map[string]*epgJob{
		source:     {guide: guide},
		"fallback": {guide: &epg.Guide{Programmes: map[string][]epg.Programme{"missing": {{}}}}},
	}
	// The same EPG can serve other subscriptions; these must not inflate this card.
	a.epg.candidates = map[string][]playlist.Channel{source: {one, missing, {Name: "Two", TVGID: "two", URL: "https://example.org/two"}}}
	summary, detail := a.subscriptionEPGSummary(sub)
	if summary != "匹配 1 / 2 个频道 · EPG 内 2 个频道" {
		t.Fatal(summary)
	}
	if strings.Contains(detail, "private") {
		t.Fatal("subscription EPG tooltip leaked a URL token")
	}
	a.epg.jobs[source].guide = &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{}}, "missing": {{}}}}
	summary, _ = a.subscriptionEPGSummary(sub)
	if summary != "匹配 2 / 2 个频道 · EPG 内 2 个频道" {
		t.Fatalf("guide replacement left stale stats: %s", summary)
	}
	sub.Channels = []playlist.Channel{one}
	sub.UpdatedAt = time.Now()
	summary, _ = a.subscriptionEPGSummary(sub)
	if summary != "匹配 1 / 1 个频道 · EPG 内 2 个频道" {
		t.Fatalf("channel refresh left stale stats: %s", summary)
	}
	a.epg.jobs[source].error = "下载失败"
	_, detail = a.subscriptionEPGSummary(sub)
	if !strings.Contains(detail, "下载失败；仍可使用已有数据") {
		t.Fatal("cached EPG failure state is not visible")
	}
}

func TestEPGPageHidesSubscriptionSourcesWithoutDroppingJobs(t *testing.T) {
	a := &application{showSettings: true, settingsEPGTab: true}
	a.epg.jobs = map[string]*epgJob{"embedded": {nextTry: time.Now().Add(time.Hour)}}
	tt := ui.NewTester(a.view, 1000, 820)
	if _, found := tt.Find("EPG 源 " + epgSourceID("embedded")); found {
		t.Fatal("subscription EPG is visible on the manual page")
	}
	if !tt.HasText("尚未添加节目单 EPG") {
		t.Fatal("manual EPG empty-state hint is missing")
	}
	if len(a.epg.jobs) != 1 {
		t.Fatal("hiding a subscription EPG removed its background job")
	}
}
