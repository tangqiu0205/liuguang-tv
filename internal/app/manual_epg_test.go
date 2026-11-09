package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestManualEPGEditorTypeSaveEditAndDelete(t *testing.T) {
	a := &application{showSettings: true, path: filepath.Join(t.TempDir(), "state.json")}
	tt := ui.NewTester(a.view, 1000, 820)
	if err := tt.Click("顶部添加自定义源"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("类型：EPG 节目单"); err != nil {
		t.Fatal(err)
	}
	if !a.subEPG || !a.settingsEPGTab {
		t.Fatal("EPG type did not select the EPG page")
	}
	local := filepath.Join(t.TempDir(), "guide.xml")
	if err := os.WriteFile(local, []byte("<tv/>"), 0600); err != nil {
		t.Fatal(err)
	}
	a.subName, a.source = "我的节目单", local
	tt.Frame()
	if err := tt.Click("添加并更新"); err != nil {
		t.Fatal(err)
	}
	entries := a.state.ManualEPGSources()
	if len(entries) != 1 || entries[0].Name != "我的节目单" || len(a.state.Subscriptions) != 0 || a.showSubEditor {
		t.Fatal("EPG source was not saved independently of channel subscriptions")
	}
	id := entries[0].ID
	if err := tt.Click("编辑 EPG " + id); err != nil {
		t.Fatal(err)
	}
	if !a.subEPG || a.source != local || a.editingEPGID != id {
		t.Fatal("EPG edit did not restore fields")
	}
	a.subName = "改名后的节目单"
	tt.Frame()
	if err := tt.Click("保存并更新"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(a.path)
	if err != nil || loaded.EPGSources[0].Name != "改名后的节目单" {
		t.Fatalf("saved EPG missing: %+v %v", loaded, err)
	}
	if err := tt.Click("删除 EPG " + id); err != nil {
		t.Fatal(err)
	}
	if len(a.state.ManualEPGSources()) != 1 {
		t.Fatal("delete did not require confirmation")
	}
	if err := tt.Click("确认删除 EPG " + id); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("尚未添加节目单 EPG") || len(a.state.ManualEPGSources()) != 0 {
		t.Fatal("delete did not restore empty state")
	}
}

func TestEPGEmptyStateIsCentered(t *testing.T) {
	a := &application{showSettings: true, settingsEPGTab: true}
	tt := ui.NewTester(a.view, 1000, 620)
	page := requiredRect(t, tt, "EPG 页面")
	label := requiredRect(t, tt, "尚未添加节目单 EPG")
	center := label.X + label.W/2
	if center < page.X+page.W/2-1 || center > page.X+page.W/2+1 {
		t.Fatal("EPG empty state is not centered")
	}
	for _, old := range []string{"备用 EPG 配置", "手动添加的节目单", "清理 EPG 缓存"} {
		if _, found := tt.Find(old); found {
			t.Fatalf("obsolete panel is still visible: %s", old)
		}
	}
	if err := tt.Click("顶部添加自定义源"); err != nil {
		t.Fatal(err)
	}
	if !a.subEPG {
		t.Fatal("EPG page add action should default to EPG type")
	}
}

func TestMultipleManualEPGFallbackOrder(t *testing.T) {
	now := time.Now()
	guide := func(id, title string) *epg.Guide {
		return &epg.Guide{Programmes: map[string][]epg.Programme{id: {{Start: now.Add(-time.Minute), Stop: now.Add(time.Hour), Title: title}}}}
	}
	a := &application{state: store.State{EPGSources: []store.EPGSourceEntry{{ID: "one", Source: "manual-one"}, {ID: "two", Source: "manual-two"}}}}
	a.epg.jobs = map[string]*epgJob{"embedded": {guide: guide("one", "订阅自带")}, "manual-one": {guide: guide("other", "其他频道")}, "manual-two": {guide: guide("one", "手动节目单")}}
	ch := playlist.Channel{TVGID: "one", URL: "https://example.org/live"}
	a.epg.byURL = map[string][]string{ch.URL: {"embedded"}}
	if got := a.scheduleAt(ch, now); len(got) != 1 || got[0].Title != "订阅自带" {
		t.Fatal("manual EPG overrode subscription EPG")
	}
	delete(a.epg.jobs, "embedded")
	if got := a.scheduleAt(ch, now); len(got) != 1 || got[0].Title != "手动节目单" {
		t.Fatal("second manual fallback was not used")
	}
}

func TestManualEPGReorderChangesFallbackAndSurvivesReload(t *testing.T) {
	now := time.Now()
	guide := func(title string) *epg.Guide {
		return &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{Start: now.Add(-time.Minute), Stop: now.Add(time.Hour), Title: title}}}}
	}
	state := store.State{EPGSources: []store.EPGSourceEntry{
		{ID: "one", Name: "First", Source: "https://example.org/first.xml"},
		{ID: "two", Name: "Second", Source: "https://example.org/second.xml"},
	}}
	a := &application{showSettings: true, settingsEPGTab: true, path: filepath.Join(t.TempDir(), "state.json"), state: state}
	a.epg.jobs = map[string]*epgJob{state.EPGSources[0].Source: {guide: guide("First")}, state.EPGSources[1].Source: {guide: guide("Second")}, "embedded": {guide: guide("Embedded")}}
	ch := playlist.Channel{TVGID: "one", URL: "https://example.org/live"}
	if got := a.scheduleAt(ch, now); len(got) != 1 || got[0].Title != "First" {
		t.Fatal("initial EPG order not honored")
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("前移 EPG two"); err != nil {
		t.Fatal(err)
	}
	if got := a.scheduleAt(ch, now); len(got) != 1 || got[0].Title != "Second" {
		t.Fatal("reordered EPG is not preferred")
	}
	a.epg.byURL = map[string][]string{ch.URL: {"embedded"}}
	if got := a.scheduleAt(ch, now); len(got) != 1 || got[0].Title != "Embedded" {
		t.Fatal("subscription EPG must stay ahead of manual EPG")
	}
	loaded, err := store.Load(a.path)
	if err != nil || len(loaded.EPGSources) != 2 || loaded.EPGSources[0].ID != "two" {
		t.Fatalf("reordered EPG was not saved: %+v %v", loaded.EPGSources, err)
	}
}

func TestManualEPGFirstUsableSourceWins(t *testing.T) {
	now := time.Now()
	ch := playlist.Channel{TVGID: "one", URL: "https://example.org/live"}
	future := &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{Start: now.Add(time.Hour), Stop: now.Add(2 * time.Hour), Title: "先排序"}}}}
	current := &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour), Title: "后排序"}}}}
	a := &application{state: store.State{EPGSources: []store.EPGSourceEntry{{ID: "one", Source: "first"}, {ID: "two", Source: "second"}}}}
	a.epg.jobs = map[string]*epgJob{"first": {guide: future}, "second": {guide: current}}
	if got := a.scheduleAt(ch, now); len(got) != 1 || got[0].Title != "先排序" {
		t.Fatal("later source displaced an earlier matching source with upcoming programmes")
	}
}

func TestManualEPGDuplicateKeepsDraft(t *testing.T) {
	a := &application{showSubEditor: true, subEPG: true, source: "https://example.org/guide.xml"}
	if _, err := a.state.UpsertEPGSource("", "first", a.source); err != nil {
		t.Fatal(err)
	}
	a.saveManualEPG("", "duplicate", a.source)
	if !a.showSubEditor || len(a.state.ManualEPGSources()) != 1 || a.source == "" || a.settingsErrors["subscription-editor"] == "" {
		t.Fatalf("duplicate EPG did not preserve draft: %+v", a.state)
	}
}
