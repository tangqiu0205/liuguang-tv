package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestSourceSwitchNativeMenuAndPlaybackScope(t *testing.T) {
	a, p, _, _ := playbackFixture(t)
	a.path = filepath.Join(t.TempDir(), "state.json")
	a.group = "全部分组"
	a.state.Subscriptions = []store.Subscription{
		{ID: "a", Name: "Source A", Channels: []playlist.Channel{{Name: "Demo A", TVGID: "demo", Group: "Group A", URL: "https://example.org/a"}, {Name: "Demo A", TVGID: "demo", Group: "Group A", URL: "https://example.org/a2"}}},
		{ID: "b", Name: "Source B", Channels: []playlist.Channel{{Name: "Demo B", TVGID: "demo", Group: "Group B", URL: "https://example.org/b"}, {Name: "Demo B", TVGID: "demo", Group: "Group B", URL: "https://example.org/b2"}, {Name: "Only B", TVGID: "other", Group: "Group B", URL: "https://example.org/other"}}},
	}
	a.state.RebuildChannels()
	a.play(a.state.SourceChannels["a"][0])
	playingURL := a.nowURL
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("切换订阅源"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"全部订阅", "1. Source A", "2. Source B"} {
		if !slices.Contains(tt.Menu(), label) {
			t.Fatalf("missing source menu item %q: %v", label, tt.Menu())
		}
	}
	status := a.status
	if err := tt.ChooseMenuItem("2. Source B"); err != nil {
		t.Fatal(err)
	}
	if a.state.SelectedSource != "b" || !tt.HasText("Source B") || a.filteredChannels().Len() != 2 {
		t.Fatal("source switch did not update library")
	}
	if a.status != status {
		t.Fatal("source switch should not overwrite playback status")
	}
	if a.nowURL != playingURL || len(p.streams) != 1 {
		t.Fatal("source switch interrupted current playback")
	}
	a.state.PreferredLines = map[string]string{playlist.ChannelKey(a.state.Channels[0]): playingURL}
	if err := tt.Click("播放 Demo B"); err != nil {
		t.Fatal(err)
	}
	if a.nowURL != "https://example.org/b" || len(playlist.Lines(a.activeChannel)) != 2 {
		t.Fatal("selected channel includes other source routes")
	}
	if err := tt.Click("切换订阅源"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("1. Source A"); err != nil {
		t.Fatal(err)
	}
	if a.filteredChannels().Len() != 1 || a.filteredChannels().At(0).Group != "Group A" {
		t.Fatal("source filter cache reused another source")
	}
	if err := tt.Click("切换订阅源"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("全部订阅"); err != nil {
		t.Fatal(err)
	}
	if a.state.SelectedSource != "" || a.filteredChannels().Len() != 2 || len(a.filteredChannels().At(0).Lines) != 4 {
		t.Fatal("all-source catalog did not restore")
	}
	a.selectLibrarySource("b")
	loaded, err := store.Load(a.path)
	if err != nil || loaded.SelectedSource != "b" {
		t.Fatal("source choice was not persisted")
	}
	a.setSubscriptionEnabled("b", false)
	if a.state.SelectedSource != "" {
		t.Fatal("disabled source did not fall back to all subscriptions")
	}
	a.selectLibrarySource("b")
	if a.state.SelectedSource != "" {
		t.Fatal("disabled source could be selected")
	}
	a.selectLibrarySource("a")
	a.deleteSubscription("a")
	if a.state.SelectedSource != "" {
		t.Fatal("deleted source did not fall back")
	}
}

func TestLongSourceTitleFitsHeader(t *testing.T) {
	a := largeChannelApp()
	a.state.Subscriptions = []store.Subscription{{ID: "long", Name: "这是一个非常长的订阅源名称用于检查频道库标题不会遮挡设置按钮", Channels: a.state.Channels}}
	a.state.RebuildChannels()
	a.state.SelectedSource = "long"
	tt := ui.NewTester(a.view, 1000, 620)
	header := requiredRect(t, tt, "频道标题")
	selector := requiredRect(t, tt, "切换订阅源")
	settings := requiredRect(t, tt, "播放器设置")
	contained(t, "订阅源切换", header, selector)
	contained(t, "设置按钮", header, settings)
	if selector.X+selector.W > settings.X+1 {
		t.Fatal("source selector overlaps settings")
	}
}
