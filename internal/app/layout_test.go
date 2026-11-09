package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func largeChannelApp() *application {
	a := &application{group: "全部分组", controls: player.Controls{Volume: 70}, state: store.State{Favorites: map[string]bool{}}}
	for i := 0; i < 1000; i++ {
		a.state.Channels = append(a.state.Channels, playlist.Channel{Name: fmt.Sprintf("测试频道 %04d", i), Group: "测试分组", URL: fmt.Sprintf("https://example.org/%d.m3u8", i)})
	}
	return a
}
func requiredRect(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.W <= 0 || r.H <= 0 {
		t.Fatalf("%s has invalid bounds: %+v", label, r)
	}
	return r
}
func contained(t *testing.T, label string, outer, inner ui.Rect) {
	t.Helper()
	const epsilon = 1
	if inner.X < outer.X-epsilon || inner.Y < outer.Y-epsilon || inner.X+inner.W > outer.X+outer.W+epsilon || inner.Y+inner.H > outer.Y+outer.H+epsilon {
		t.Errorf("%s escapes container: outer=%+v inner=%+v", label, outer, inner)
	}
}
func TestLargePlaylistLayout(t *testing.T) {
	for _, size := range [][2]int{{1280, 820}, {1000, 620}, {1600, 1000}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a := largeChannelApp()
			tt := ui.NewTester(a.view, size[0], size[1])
			check := func() {
				body := requiredRect(t, tt, "主分栏")
				root := requiredRect(t, tt, "应用界面")
				if body.X != 0 || body.W != root.W || body.Y != 0 || body.H != root.H {
					t.Errorf("unexpected outer whitespace: root=%+v body=%+v", root, body)
				}
				if _, exists := tt.Find("状态栏"); exists {
					t.Error("standalone status bar should be removed")
				}
				contained(t, "状态提示", requiredRect(t, tt, "播放控制栏"), requiredRect(t, tt, "状态提示"))
				if _, exists := tt.Find("应用栏"); exists {
					t.Error("brand header should be removed")
				}
				channels := requiredRect(t, tt, "频道面板")
				list := requiredRect(t, tt, "频道列表")
				panel := requiredRect(t, tt, "播放面板")
				video := requiredRect(t, tt, "视频画面")
				if channels.X != body.X || channels.Y != body.Y || channels.H != body.H || panel.Y != body.Y || panel.H != body.H || panel.X+panel.W != body.X+body.W {
					t.Errorf("playback panels should be flush with the window edges: channels=%+v panel=%+v body=%+v", channels, panel, body)
				}
				for label, r := range map[string]ui.Rect{"频道面板": channels, "播放面板": panel} {
					contained(t, label, body, r)
				}
				contained(t, "频道列表", channels, list)
				contained(t, "视频画面", panel, video)
				contained(t, "设置入口", channels, requiredRect(t, tt, "播放器设置"))
				title := requiredRect(t, tt, "频道标题")
				icon := requiredRect(t, tt, "应用图标")
				contained(t, "应用图标", title, icon)
				if icon.X+icon.W > requiredRect(t, tt, "切换订阅源").X {
					t.Error("application icon should be left of the source selector")
				}
				if video.H < body.H-155 {
					t.Errorf("video collapsed: body=%+v video=%+v", body, video)
				}
				if panel.X != channels.X+channels.W {
					t.Errorf("unexpected gap between panels: channels=%+v video=%+v", channels, panel)
				}
				if panel.X < channels.X+channels.W {
					t.Errorf("panels overlap: channels=%+v video=%+v", channels, panel)
				}
				first, last := a.list.Visible()
				if last-first > 60 {
					t.Errorf("not virtualized: visible %d..%d", first, last)
				}
			}
			check()
			list := requiredRect(t, tt, "频道列表")
			tt.Wheel(list.X+list.W/2, list.Y+list.H/2, 0, 700)
			tt.Advance(time.Second)
			check()
			first, _ := a.list.Visible()
			if first == 0 {
				t.Error("wheel did not scroll channel list")
			}
			tt.SetSize(size[0]+120, size[1]+80)
			check()
			// Match the screenshot's 150% Windows scaling while preserving logical size.
			tt.SetSize(size[0]*3/2, size[1]*3/2)
			tt.SetScale(1.5)
			check()
		})
	}
}
