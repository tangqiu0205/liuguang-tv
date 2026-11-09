package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"path/filepath"
	"testing"
)

func TestResolveBuildVersion(t *testing.T) {
	for _, tc := range []struct {
		build, packaged, want string
		wantErr               bool
	}{
		{"", "", "dev", false},
		{"dev", "", "dev", false},
		{"dev", "1.2.3", "1.2.3", false},
		{"1.2.3", "", "1.2.3", false},
		{"1.2.3", "1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.4", "", true},
	} {
		got, err := resolveBuildVersion(tc.build, tc.packaged)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("resolveBuildVersion(%q, %q) = %q, %v", tc.build, tc.packaged, got, err)
		}
	}
}

func TestBuildVersionOnlyAppearsOnAboutPage(t *testing.T) {
	for _, value := range []string{"", "1.2.3"} {
		a := &application{version: value, showSettings: true, settingsAppearanceTab: true}
		want := value
		if want == "" {
			want = "dev"
		}
		tt := ui.NewTester(a.view, 1000, 620)
		if tt.HasText("流光 TV · " + want) {
			t.Fatal("version footer should not appear in appearance settings")
		}
		if err := tt.Click("关于"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText("版本：" + want) {
			t.Fatal("command-supplied version was not displayed on about page")
		}
	}
}

func TestCinemaView(t *testing.T) {
	a := &application{group: "全部分组", state: store.State{Favorites: map[string]bool{}}}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("全屏"); err != nil {
		t.Fatal(err)
	}
	if !a.cinema || !tt.HasText("退出全屏") {
		t.Fatal("did not enter cinema layout")
	}
	if err := tt.Click("退出全屏"); err != nil {
		t.Fatal(err)
	}
	if a.cinema || !tt.HasText("暂无频道") {
		t.Fatal("did not restore channel list")
	}
}

func TestPlaybackControlsView(t *testing.T) {
	a := &application{group: "全部分组", now: "Test", controls: player.Controls{Volume: 70}, state: store.State{Favorites: map[string]bool{}}}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("停止"); err != nil {
		t.Fatal(err)
	}
	if a.now != "" || !tt.HasText("视频播放区域") {
		t.Fatal("did not stop playback")
	}
}

func TestEmptyView(t *testing.T) {
	a := &application{group: "全部分组", state: store.State{Favorites: map[string]bool{}}}
	tt := ui.NewTester(a.view, 1080, 760)
	if !tt.HasText("暂无频道") {
		t.Fatal(tt.Texts())
	}
}
func TestFavoriteAndSearch(t *testing.T) {
	a := &application{group: "全部分组", path: filepath.Join(t.TempDir(), "state.json"), state: store.State{
		Channels: []playlist.Channel{{Name: "Test News", Group: "News", URL: "https://example.org/live"}}, Favorites: map[string]bool{},
	}}
	tt := ui.NewTester(a.view, 1080, 760)
	if !tt.HasText("Test News") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	if !a.state.Favorites["https://example.org/live"] {
		t.Fatal("not favorited")
	}
	a.query = "no match"
	tt = ui.NewTester(a.view, 1080, 760)
	if !tt.HasText("暂无频道") {
		t.Fatal(tt.Texts())
	}
}
