package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func cardDemo() *application {
	a := &application{showSettings: true, state: store.State{Favorites: map[string]bool{}}}
	for i, name := range []string{"TianmuTNT", "myIPTV", "zbds", "来源于网络"} {
		id, _ := a.state.UpsertSubscription("", name, fmt.Sprintf("https://example.org/%d.m3u", i))
		counts := []int{663, 1273, 530, 40}
		channels := make([]playlist.Channel, counts[i])
		for j := range channels {
			channels[j] = playlist.Channel{Name: fmt.Sprintf("演示频道 %d", j), URL: fmt.Sprintf("https://example.org/%d/%d", i, j)}
		}
		a.state.ApplyRefresh(id, 1, channels, nil, time.Now())
	}
	return a
}
func TestSubscriptionCardLayout(t *testing.T) {
	for _, width := range []int{1000, 1280, 2000} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := cardDemo()
			a.path = filepath.Join(t.TempDir(), "state.json")
			tt := ui.NewTester(a.view, width, 900)
			grid := requiredRect(t, tt, "订阅卡片网格")
			if _, found := tt.Find("添加自定义源"); found {
				t.Fatal("add-source placeholder card is still visible")
			}
			for _, sub := range a.state.Subscriptions {
				card := requiredRect(t, tt, "订阅卡片 "+sub.ID)
				contained(t, sub.Name, grid, card)
				if card.H != 200 || card.W < 260 || card.W > 410 {
					t.Fatalf("invalid card size: %+v", card)
				}
				switcher := requiredRect(t, tt, "停用订阅 "+sub.Name)
				refresh := requiredRect(t, tt, "刷新订阅 "+sub.Name)
				if switcher.X < card.X+card.W/2 || switcher.X+switcher.W > refresh.X {
					t.Fatalf("subscription switch is not aligned with right-side actions: %+v", switcher)
				}
				contained(t, "刷新", card, refresh)
				contained(t, "删除", card, requiredRect(t, tt, "删除订阅 "+sub.Name))
				contained(t, "订阅更新时间", card, requiredRect(t, tt, "订阅更新时间 "+sub.ID))
				contained(t, "订阅 EPG 统计", card, requiredRect(t, tt, "订阅 EPG 统计 "+sub.ID))
				if !tt.HasText("最近更新：" + sub.UpdatedAt.Local().Format("2006-01-02 15:04")) {
					t.Fatal("update time is not visible")
				}
				for _, prefix := range []string{"编辑订阅 ", "停用订阅 ", "前移订阅 ", "后移订阅 "} {
					contained(t, prefix, card, requiredRect(t, tt, prefix+sub.Name))
				}
			}
			// Reordering is an actual persisted operation, not a decorative arrow.
			if err := tt.Click("前移订阅 myIPTV"); err != nil {
				t.Fatal(err)
			}
			if a.state.Subscriptions[0].Name != "myIPTV" {
				t.Fatal("move did not reorder subscriptions")
			}
			if err := tt.Click("停用订阅 myIPTV"); err != nil {
				t.Fatal(err)
			}
			if !a.state.Subscriptions[0].Disabled {
				t.Fatal("disable did not apply")
			}
			loaded, err := store.Load(a.path)
			if err != nil || loaded.Subscriptions[0].Name != "myIPTV" || !loaded.Subscriptions[0].Disabled {
				t.Fatalf("card changes not saved: %v", err)
			}
		})
	}
}
func TestSubscriptionEnableDisableButtons(t *testing.T) {
	a := &application{showSettings: true, path: filepath.Join(t.TempDir(), "state.json"), state: store.State{Subscriptions: []store.Subscription{{ID: "one", Name: "演示订阅"}}}}
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("停用订阅 演示订阅"); err != nil {
		t.Fatal(err)
	}
	if !a.state.Subscriptions[0].Disabled {
		t.Fatal("disable button did not disable the subscription")
	}
	if err := tt.Click("启用订阅 演示订阅"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(a.path)
	if err != nil || loaded.Subscriptions[0].Disabled {
		t.Fatalf("enable button did not persist the enabled state: %v", err)
	}
}

func TestSubscriptionUpdateTimeRetainsLastSuccess(t *testing.T) {
	a := cardDemo()
	sub := &a.state.Subscriptions[0]
	last := time.Date(2026, 10, 20, 10, 30, 0, 0, time.Local)
	sub.UpdatedAt = last
	if !a.state.ApplyRefresh(sub.ID, sub.Revision, nil, fmt.Errorf("download failed"), last.Add(time.Hour)) {
		t.Fatal("failure was not applied")
	}
	_, err := a.state.UpsertSubscription("", "New", "https://example.org/new.m3u")
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1280, 1000)
	if !tt.HasText("最近更新：2026-10-20 10:30") || !tt.HasText("最近更新：尚未更新") {
		t.Fatal("missing last successful / never updated timestamps")
	}
	if !tt.HasText("更新失败：download failed") {
		t.Fatal("refresh failure reason is not visible on the card")
	}
	if tt.HasText("最近更新：2026-10-20 11:30") {
		t.Fatal("failed check replaced successful update time")
	}
}

func TestSubscriptionSideEditor(t *testing.T) {
	for _, width := range []int{1000, 1280, 2000} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := cardDemo()
			a.path = filepath.Join(t.TempDir(), "state.json")
			tt := ui.NewTester(a.view, width, 900)
			if err := tt.Click("顶部添加自定义源"); err != nil {
				t.Fatal(err)
			}
			pane := requiredRect(t, tt, "订阅编辑栏")
			left := requiredRect(t, tt, "设置列表")
			content := requiredRect(t, tt, "设置分栏")
			if pane.X != left.X+left.W || pane.X+pane.W != content.X+content.W || pane.Y != left.Y || pane.H != left.H {
				t.Fatalf("editor is not docked to the right: list=%+v editor=%+v", left, pane)
			}
			for _, label := range []string{"订阅名称", "订阅地址或文件路径", "添加并更新", "取消添加", "选择本地文件…", "关闭订阅编辑"} {
				contained(t, label, pane, requiredRect(t, tt, label))
			}
			source := requiredRect(t, tt, "订阅地址或文件路径")
			pick := requiredRect(t, tt, "选择本地文件…")
			save := requiredRect(t, tt, "添加并更新")
			if pick.Y < source.Y+source.H || save.Y < pick.Y+pick.H {
				t.Fatal("file picker must follow the source field and precede saving")
			}
			if !tt.HasText("订阅地址或文件路径") || !tt.HasText("订阅名称（可选）") {
				t.Fatal("source and name need visible labels, not only placeholders")
			}
			before := requiredRect(t, tt, "订阅名称")
			tt.Scroll(left.X+left.W/2, left.Y+left.H/2, 0, 300)
			if after := requiredRect(t, tt, "订阅名称"); after != before {
				t.Fatal("list scrolling moved the editor")
			}
			if err := tt.Click("关闭订阅编辑"); err != nil {
				t.Fatal(err)
			}
			if _, visible := tt.Find("订阅编辑栏"); visible {
				t.Fatal("close did not collapse sidebar")
			}
			a.settingsScroll.ScrollTo(0, 0, ui.Animation{})
			tt.Frame()
			if err := tt.Click("编辑订阅 TianmuTNT"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("保存并更新") || a.editingID != a.state.Subscriptions[0].ID {
				t.Fatal("edit did not populate sidebar")
			}
			// Duplicate source validation must keep the sidebar and draft open.
			a.source = a.state.Subscriptions[1].Source
			tt.Frame()
			if err := tt.Click("保存并更新"); err != nil {
				t.Fatal(err)
			}
			if !a.showSubEditor || a.source != a.state.Subscriptions[1].Source {
				t.Fatal("failed save discarded the draft")
			}
			if err := tt.Click("取消编辑"); err != nil {
				t.Fatal(err)
			}
			if a.showSubEditor || a.editingID != "" {
				t.Fatal("cancel did not close editor")
			}
		})
	}
}

func TestSubscriptionEditorFooterStaysVisible(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			a := &application{showSettings: true, state: store.State{ThemeMode: mode}}
			tt := ui.NewTester(a.view, 1000, 620)
			if err := tt.Click("顶部添加自定义源"); err != nil {
				t.Fatal(err)
			}
			pane := requiredRect(t, tt, "订阅编辑栏")
			footer := requiredRect(t, tt, "订阅编辑操作区")
			for _, label := range []string{"添加并更新", "取消添加", "关闭订阅编辑"} {
				contained(t, label, pane, requiredRect(t, tt, label))
			}
			fields := requiredRect(t, tt, "编辑栏滚动区")
			tt.Scroll(fields.X+fields.W/2, fields.Y+fields.H/2, 0, 500)
			if a.editorScroll.Y <= 0 {
				t.Fatal("test did not scroll editor fields")
			}
			if after := requiredRect(t, tt, "订阅编辑操作区"); after != footer {
				t.Fatal("scrolling fields moved footer actions")
			}
		})
	}
}

func TestSubscriptionCardsPreview(t *testing.T) {
	path := os.Getenv("LIUGUANG_TV_SUBSCRIPTIONS_PREVIEW")
	if path == "" {
		t.Skip("set LIUGUANG_TV_SUBSCRIPTIONS_PREVIEW to export a card preview")
	}
	a := cardDemo()
	for i := 4; i < 8; i++ {
		_, err := a.state.UpsertSubscription("", fmt.Sprintf("订阅源 %d", i+1), fmt.Sprintf("https://example.org/%d.m3u", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	tt := ui.NewTester(a.view, 1500, 900)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	if editorPath := os.Getenv("LIUGUANG_TV_EDITOR_PREVIEW"); editorPath != "" {
		if err := tt.Click("编辑订阅 myIPTV"); err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(editorPath)
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		if err := png.Encode(out, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
