package app

import (
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestEPGMatchingOrderHint(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		t.Run(tag, func(t *testing.T) {
			useTestLanguage(t, tag)
			a := &application{showSettings: true, settingsEPGTab: true}
			tt := ui.NewTester(a.view, 1000, 620)
			hint := tr("优先使用订阅自带节目单；无可用匹配时，按下方来源顺序依次匹配，可用左右箭头调整优先级。")
			r := requiredRect(t, tt, hint)
			tabs := requiredRect(t, tt, tr("订阅源子标签栏"))
			contained(t, "EPG matching order hint", requiredRect(t, tt, tr("订阅源区域标题")), r)
			if r.Y < tabs.Y+tabs.H {
				t.Fatal("matching order hint overlaps the tabs")
			}
			if err := tt.Click(tr("订阅管理")); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(hint) {
				t.Fatal("EPG matching order hint leaked onto subscription management")
			}
		})
	}
}

func TestEPGEditingKeepsTwoColumnGrid(t *testing.T) {
	a := &application{showSettings: true, settingsEPGTab: true}
	for i := 0; i < 4; i++ {
		a.state.EPGSources = append(a.state.EPGSources, store.EPGSourceEntry{ID: fmt.Sprint(i), Name: fmt.Sprintf("EPG %d", i), Source: fmt.Sprintf("https://example.org/%d.xml", i)})
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("编辑 EPG 0"); err != nil {
		t.Fatal(err)
	}
	if a.editingEPGID != "0" {
		t.Fatal("editing EPG is not tracked for selection highlighting")
	}
	first := requiredRect(t, tt, "EPG 源 "+epgSourceID(a.state.EPGSources[0].Source))
	second := requiredRect(t, tt, "EPG 源 "+epgSourceID(a.state.EPGSources[1].Source))
	if first.Y != second.Y || second.X < first.X+first.W {
		t.Fatalf("EPG cards should occupy separate columns while editing: first=%+v second=%+v", first, second)
	}
	for i := 0; i < 2; i++ {
		id := a.state.EPGSources[i].ID
		card := requiredRect(t, tt, "EPG 源 "+epgSourceID(a.state.EPGSources[i].Source))
		for _, action := range []string{"编辑 EPG ", "前移 EPG ", "后移 EPG "} {
			contained(t, action, card, requiredRect(t, tt, action+id))
		}
	}
	pane := requiredRect(t, tt, "订阅编辑栏")
	if second.X+second.W > pane.X {
		t.Fatal("EPG cards overlap the editor")
	}
}

func TestEPGSettingsCardGrid(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		for _, scale := range []float32{1, 1.5} {
			t.Run(fmt.Sprintf("%s/%v", mode, scale), func(t *testing.T) {
				a := &application{showSettings: true, settingsEPGTab: true, state: store.State{ThemeMode: mode}}
				a.epg.jobs = make(map[string]*epgJob)
				for i := 0; i < 8; i++ {
					a.epg.jobs[fmt.Sprintf("https://epg%d.example.org/guide.xml?token=private", i)] = &epgJob{updated: time.Now(), nextTry: time.Now().Add(24 * time.Hour)}
				}
				a.state.EPGSource = "https://epg0.example.org/guide.xml?token=private"
				tt := ui.NewTester(a.view, int(1000*scale), int(820*scale))
				tt.SetScale(scale)
				grid := requiredRect(t, tt, "EPG 卡片网格")
				var cards []ui.Rect
				for source := range a.epg.jobs {
					id := epgSourceID(source)
					if source != a.state.EPGSource {
						if _, found := tt.Find("EPG 源 " + id); found {
							t.Fatal("subscription EPG leaked into manual EPG page")
						}
						continue
					}
					card := requiredRect(t, tt, "EPG 源 "+id)
					contained(t, "EPG card", grid, card)
					contained(t, "refresh", card, requiredRect(t, tt, "刷新 EPG 源 "+id))
					for _, previous := range cards {
						if card.X < previous.X+previous.W && card.X+card.W > previous.X && card.Y < previous.Y+previous.H && card.Y+card.H > previous.Y {
							t.Fatal("EPG source cards overlap")
						}
					}
					cards = append(cards, card)
				}
				if _, found := tt.Find("备用 EPG 配置"); found {
					t.Fatal("obsolete EPG configuration panel is still visible")
				}
				if tt.HasText("token=private") {
					t.Fatal("EPG source URL credentials leaked into cards")
				}
				if path := os.Getenv("LIUGUANG_TV_EPG_SETTINGS_PREVIEW"); path != "" && mode == "light" && scale == 1 {
					f, err := os.Create(path)
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					closeErr := f.Close()
					if err != nil {
						t.Fatal(err)
					}
					if closeErr != nil {
						t.Fatal(closeErr)
					}
				}
			})
		}
	}
}
