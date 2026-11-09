package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestChannelFilterActions(t *testing.T) {
	a := largeChannelApp()
	tt := ui.NewTester(a.view, 1000, 620)
	group := requiredRect(t, tt, "频道分组")
	count := requiredRect(t, tt, "1000 个频道")
	contained(t, "频道数量", group, count)
	if err := tt.Click("频道分组"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("频道分组选项 测试分组"); err != nil {
		t.Fatal(err)
	}
	if a.group != "测试分组" {
		t.Fatal("group selection failed")
	}
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Type("0001")
	if err := tt.Click("清除关键词"); err != nil {
		t.Fatal(err)
	}
	if a.query != "" || !a.showSearch || a.group != "测试分组" || a.filteredChannels().Len() != 1000 {
		t.Fatal("keyword clear failed or changed group")
	}
	if tt.HasText("已筛选") || tt.HasText("清除所有筛选") {
		t.Fatal("unexpected filter controls")
	}
}

func TestSearchMatchesChannelNameOnly(t *testing.T) {
	a := largeChannelApp()
	a.query = "测试分组"
	if got := a.filteredChannels().Len(); got != 0 {
		t.Fatalf("group name matched %d channels", got)
	}
	a.query = "频道 0001"
	if got := a.filteredChannels().Len(); got != 1 {
		t.Fatalf("channel name matched %d channels, want 1", got)
	}
}

func TestChannelEmptyAndPlayingStates(t *testing.T) {
	a := largeChannelApp()
	a.activeChannel = a.state.Channels[0]
	a.now = "playing"
	tt := ui.NewTester(a.view, 1000, 620)
	if !tt.HasText("播放中") {
		t.Fatal("missing playing indicator")
	}
	a.state.Channels = nil
	a.now = ""
	tt = ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("管理订阅源"); err != nil {
		t.Fatal(err)
	}
	if !a.showSettings || !tt.HasText("订阅源") {
		t.Fatal("subscription management did not open")
	}
}

func TestSearchIconToggleAndEscape(t *testing.T) {
	a := largeChannelApp()
	tt := ui.NewTester(a.view, 1000, 620)
	if _, visible := tt.Find("搜索频道"); visible {
		t.Fatal("search field should start collapsed")
	}
	bar := requiredRect(t, tt, "频道筛选栏")
	button := requiredRect(t, tt, "搜索")
	contained(t, "搜索按钮", bar, button)
	favorites := requiredRect(t, tt, "我的收藏")
	if button.X < favorites.X+favorites.W {
		t.Fatal("search should be on the right of the favorites tab")
	}
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if !a.showSearch || !tt.Focused("搜索频道") {
		t.Fatal("search did not open with input focus")
	}
	if _, visible := tt.Find("频道分组"); visible {
		t.Fatal("group selector should be hidden during search")
	}
	tt.Type("0001")
	if a.filteredChannels().Len() != 1 {
		t.Fatal("search did not filter channels")
	}
	if err := tt.Click("关闭搜索"); err != nil {
		t.Fatal(err)
	}
	if a.showSearch || a.query != "" || a.filteredChannels().Len() != 1000 {
		t.Fatal("closing search did not clear filter")
	}
	if _, visible := tt.Find("搜索频道"); visible {
		t.Fatal("search field still reserves space")
	}
	requiredRect(t, tt, "频道分组")
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Type("no matching channel")
	if !tt.HasText("暂无频道") {
		t.Fatal("unmatched query should show no channels")
	}
	tt.Key(0, ui.KeyEscape)
	if a.showSearch || a.query != "" || a.filteredChannels().Len() != 1000 {
		t.Fatal("Escape did not dismiss search")
	}
}
