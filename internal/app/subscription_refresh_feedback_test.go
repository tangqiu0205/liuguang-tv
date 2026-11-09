package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestSingleRefreshFeedbackUsesRefreshedSubscription(t *testing.T) {
	a := &application{
		path: filepath.Join(t.TempDir(), "state.json"),
		state: store.State{Subscriptions: []store.Subscription{
			{ID: "one", Name: "当前源", Revision: 1},
			{ID: "old", Name: "其他源", LastError: "之前更新失败", Channels: []playlist.Channel{{Name: "其他", URL: "https://example.org/other"}}},
		}},
		refreshing: map[string]uint64{"one": 1},
	}
	channels := []playlist.Channel{{Name: "一", URL: "https://example.org/1"}, {Name: "二", URL: "https://example.org/2"}}
	a.completeRefresh("one", 1, 1, channels, nil)
	if a.settingsToast != "订阅 当前源 更新完成，共 2 个频道" {
		t.Fatalf("unexpected single refresh feedback: %q", a.settingsToast)
	}
	if a.state.Subscriptions[1].LastError == "" {
		t.Fatal("unrelated failure was cleared")
	}
}

func TestRefreshBatchFeedbackCountsOnlyCurrentResults(t *testing.T) {
	a := &application{
		path:       filepath.Join(t.TempDir(), "state.json"),
		state:      store.State{Subscriptions: []store.Subscription{{ID: "one", Name: "一", Revision: 1}, {ID: "two", Name: "二", Revision: 1}, {ID: "old", Name: "旧", LastError: "历史失败"}}},
		refreshing: map[string]uint64{"one": 1, "two": 2},
	}
	a.completeRefresh("one", 1, 1, []playlist.Channel{{Name: "一", URL: "https://example.org/1"}}, nil)
	if a.settingsToast != "" {
		t.Fatal("batch reported completion before all refreshes finished")
	}
	a.completeRefresh("two", 2, 1, nil, errors.New("network"))
	if a.status != "本次更新 2 个订阅，1 个失败（保留上次频道缓存）" || a.settingsToast != "" {
		t.Fatalf("incorrect partial failure feedback: %s / %s", a.status, a.settingsToast)
	}
}

func TestRefreshFeedbackWaitsForSuccessfulSave(t *testing.T) {
	a := &application{path: filepath.Join(t.TempDir(), "missing", "state.json"), state: store.State{Subscriptions: []store.Subscription{{ID: "one", Name: "测试", Revision: 1}}}, refreshing: map[string]uint64{"one": 1}}
	// A regular file used as a parent prevents writing the state file.
	a.path = filepath.Join(t.TempDir(), "state.json", "nested")
	if err := store.Save(filepath.Dir(a.path), store.State{}); err != nil {
		t.Fatal(err)
	}
	a.completeRefresh("one", 1, 1, []playlist.Channel{{Name: "一", URL: "https://example.org/1"}}, nil)
	if a.settingsToast != "" || !strings.Contains(a.status, "保存设置失败") {
		t.Fatalf("failed write was reported as success: %s / %s", a.status, a.settingsToast)
	}
}
