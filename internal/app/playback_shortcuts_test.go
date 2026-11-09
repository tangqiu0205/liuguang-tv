package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"testing"
)

func TestPlaybackShortcutsAndTextEntryIsolation(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	a.state.Channels = []playlist.Channel{ch, {Name: "Second", Group: "Other", URL: "https://example.org/second"}}
	a.group = "全部分组"
	a.play(ch)
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("播放 Demo"); err != nil {
		t.Fatal(err)
	}
	started := len(p.streams)
	if embedhost.Supported {
		tt.Key(0, ui.KeySpace)
		if !a.controls.Paused || len(p.streams) != started {
			t.Fatal("Space replayed focused channel instead of pausing")
		}
		tt.Key(0, ui.KeySpace)
		if a.controls.Paused {
			t.Fatal("Space did not resume")
		}
		tt.Key(0, ui.KeyRight)
		if a.controls.Volume != 75 {
			t.Fatalf("volume=%v", a.controls.Volume)
		}
		tt.Key(0, ui.KeyLeft)
		if a.controls.Volume != 70 {
			t.Fatal("volume did not decrease")
		}
	}
	tt.Key(0, ui.KeyDown)
	if a.now != "Second" {
		t.Fatalf("Down did not switch channel: %s", a.now)
	}
	tt.Key(0, ui.KeyUp)
	if a.now != "Demo" {
		t.Fatal("Up did not switch back")
	}
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	before := len(p.streams)
	paused := a.controls.Paused
	volume := a.controls.Volume
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyRight)
	if len(p.streams) != before || a.controls.Paused != paused || a.controls.Volume != volume {
		t.Fatal("search keys triggered playback")
	}
	a.showSearch = false
	a.query = ""
	a.showSettings = true
	tt.SetSize(1280, 820)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeySpace)
	if len(p.streams) != before || a.controls.Paused != paused {
		t.Fatal("settings keys triggered playback")
	}
}
func TestStepChannelRespectsFilteredSourceAndWraps(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	a.group = "全部分组"
	a.state.Channels = []playlist.Channel{ch, {Name: "Other", Group: "Sports", URL: "https://example.org/other"}}
	a.query = "Other"
	a.stepChannel(1)
	if a.now != "Other" {
		t.Fatal("selection ignored current filter")
	}
	a.stepChannel(-1)
	if len(p.streams) != 2 || a.now != "Other" {
		t.Fatal("single-item wrap changed scope")
	}
	a.query = "no matches"
	a.stepChannel(1)
	if len(p.streams) != 2 {
		t.Fatal("empty selection started a stream")
	}
}
