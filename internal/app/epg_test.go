package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestSaveEPGBlockedWhenConfigRecoveryFailed(t *testing.T) {
	a := &application{
		configError: errors.New("backup restore failed"),
		state:       store.State{EPGSource: "https://old.example/guide.xml"},
		source:      "https://new.example/guide.xml",
	}
	a.saveManualEPG("", "", a.source)
	if a.state.EPGSource != "https://old.example/guide.xml" || a.source != "https://new.example/guide.xml" || len(a.state.EPGSources) != 0 {
		t.Fatal("blocked save changed persisted or draft EPG settings")
	}
	if !strings.Contains(a.status, "未保存") || strings.Contains(a.status, "EPG 设置已保存") {
		t.Fatalf("blocked save reported success: %s", a.status)
	}
}

func TestSchedulePrefersM3UThenManual(t *testing.T) {
	guide := func(title string) *epg.Guide {
		g, err := epg.Parse(strings.NewReader(`<tv><programme channel="one" start="20260801190000 +0800" stop="20260801200000 +0800"><title>` + title + `</title></programme></tv>`))
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	a := &application{state: store.State{EPGSource: "manual"}}
	a.epg.jobs = map[string]*epgJob{"m3u": {guide: guide("M3U")}, "manual": {guide: guide("手动")}}
	a.epg.byURL = map[string][]string{"https://example.com/live": {"m3u"}}
	ch := playlist.Channel{TVGID: "one", URL: "https://example.com/live"}
	if ps := a.schedule(ch); len(ps) != 1 || ps[0].Title != "M3U" {
		t.Fatal("M3U did not win")
	}
	a.epg.jobs["m3u"].guide = nil
	if ps := a.schedule(ch); len(ps) != 1 || ps[0].Title != "手动" {
		t.Fatal("manual fallback did not work")
	}
	delete(a.epg.jobs, "manual")
	if len(a.schedule(ch)) != 0 {
		t.Fatal("unexpected schedule")
	}
}

func TestEPGLayout(t *testing.T) {
	for _, width := range []int{1000, 1280} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := largeChannelApp()
			a.activeChannel = a.state.Channels[0]
			a.now = a.activeChannel.Name
			a.epg.show = true
			tt := ui.NewTester(a.view, width, 820)
			body := requiredRect(t, tt, "主分栏")
			for _, label := range []string{"频道面板", "播放面板", "EPG 节目单"} {
				contained(t, label, body, requiredRect(t, tt, label))
			}
			panel := requiredRect(t, tt, "播放面板")
			for _, label := range []string{"视频画面", "节目摘要", "播放控制栏"} {
				contained(t, label, panel, requiredRect(t, tt, label))
			}
			tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
			bar := requiredRect(t, tt, "沉浸顶栏")
			for _, label := range []string{"节目摘要", "播放频道台标", "播放频道名", "当前节目", "下一个节目", "节目单"} {
				contained(t, label, bar, requiredRect(t, tt, label))
			}
			name := requiredRect(t, tt, "播放频道名")
			current := requiredRect(t, tt, "当前节目")
			next := requiredRect(t, tt, "下一个节目")
			divider := requiredRect(t, tt, "频道节目分隔线")
			if name.W < 32 || current.W < 32 || next.W < 32 {
				t.Fatal("narrow playback header must retain readable channel and program columns")
			}
			if divider.W != 2 || divider.X < name.X+name.W || divider.X+divider.W > current.X {
				t.Fatal("separator must sit between the channel name and programme details")
			}
			logo := requiredRect(t, tt, "播放频道台标")
			if logo.X != bar.X+16 || name.X < logo.X+logo.W || current.X <= name.X || current.X != next.X {
				t.Fatal("channel logo and name must precede the aligned programme lines")
			}
			video := requiredRect(t, tt, "视频画面")
			controls := requiredRect(t, tt, "播放控制栏")
			if video.Y != bar.Y+bar.H || video.Y+video.H != controls.Y {
				t.Fatal("summary still consumes video space")
			}
			if err := tt.Click("关闭节目单"); err != nil {
				t.Fatal("cannot close EPG")
			}
			tt.Advance(time.Millisecond * 20)
			if _, ok := tt.Find("EPG 节目单"); ok {
				t.Fatal("EPG did not close")
			}
			for _, button := range []string{"窗口全屏", "全屏"} {
				if err := tt.Click(button); err != nil {
					t.Fatal(err)
				}
				assertNoFullscreenHeader(t, tt)
				requiredRect(t, tt, "播放控制栏")
			}
		})
	}
}

func TestEPGMetadataPersists(t *testing.T) {
	path := t.TempDir() + "/state.json"
	state := store.State{EPGSource: "https://example.com/fallback.xml", Subscriptions: []store.Subscription{{ID: "one", Source: "https://example.com/list.m3u", EPGURLs: []string{"https://example.com/epg.xml"}}}}
	if err := store.Save(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EPGSource != state.EPGSource || len(loaded.Subscriptions[0].EPGURLs) != 1 {
		t.Fatal("lost EPG metadata")
	}
	if _, err := loaded.UpsertSubscription("one", "renamed", state.Subscriptions[0].Source); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Subscriptions[0].EPGURLs) != 1 {
		t.Fatal("rename lost EPG")
	}
	if _, err := loaded.UpsertSubscription("one", "renamed", "https://example.com/new.m3u"); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Subscriptions[0].EPGURLs) != 0 {
		t.Fatal("source change retained stale EPG")
	}
}
