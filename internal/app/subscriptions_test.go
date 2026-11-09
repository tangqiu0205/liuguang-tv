package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestRefreshCompletionRejectsStaleRequest(t *testing.T) {
	s := store.State{Favorites: map[string]bool{}}
	id, err := s.UpsertSubscription("", "One", "https://example.org/list")
	if err != nil {
		t.Fatal(err)
	}
	a := &application{state: s, path: filepath.Join(t.TempDir(), "state.json"), group: "Old Group", refreshing: map[string]uint64{id: 2}, cancelLoads: map[string]context.CancelFunc{}}
	ch := []playlist.Channel{{Name: "New", Group: "New Group", URL: "https://example.org/live"}}
	a.completeRefresh(id, 1, 1, ch, nil)
	if len(a.state.Channels) != 0 || a.refreshing[id] != 2 {
		t.Fatal("older request overwrote active request")
	}
	a.completeRefresh(id, 2, 1, ch, nil)
	if len(a.state.Channels) != 1 || a.group != "全部分组" || len(a.refreshing) != 0 {
		t.Fatal("successful refresh not applied")
	}
	a.refreshing[id] = 3
	a.completeRefresh(id, 3, 1, nil, fmt.Errorf("HTTP 503"))
	if len(a.state.Channels) != 1 || a.state.Subscriptions[0].LastError == "" {
		t.Fatal("failure did not preserve cache")
	}
}
func TestSubscriptionEditorAndDelete(t *testing.T) {
	a := &application{path: filepath.Join(t.TempDir(), "state.json"), showSettings: true, state: store.State{Favorites: map[string]bool{}}}
	id, err := a.state.UpsertSubscription("", "Local Test", "https://example.org/list?secret=hidden")
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1280, 1000)
	if err := tt.Click("编辑订阅 Local Test"); err != nil {
		t.Fatal(err)
	}
	if a.editingID != id || a.subName != "Local Test" || a.source != "https://example.org/list?secret=hidden" || !tt.HasText("保存并更新") {
		t.Fatal("editor did not populate")
	}
	if err := tt.Click("取消编辑"); err != nil {
		t.Fatal(err)
	}
	if a.editingID != "" || a.source != "" {
		t.Fatal("editor did not clear")
	}
	if err := tt.Click("删除订阅 Local Test"); err != nil {
		t.Fatal(err)
	}
	if len(a.state.Subscriptions) != 1 {
		t.Fatal("deleted without confirmation")
	}
	if err := tt.Click("确认删除"); err != nil {
		t.Fatal(err)
	}
	if len(a.state.Subscriptions) != 0 {
		t.Fatal("delete not applied")
	}
	if subscriptionLocation("https://user:password@example.org/list?secret=hidden") != "https://example.org/list" {
		t.Fatal("summary leaks credentials")
	}
}
