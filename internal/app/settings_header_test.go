package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsHeaderUsesCompactTopSpacing(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		for _, height := range []float32{0, 32, 40, 64} {
			t.Run(fmt.Sprintf("%s/caption=%v", tag, height), func(t *testing.T) {
				useTestLanguage(t, tag)
				a := &application{showSettings: true}
				tt := ui.NewTester(a.view, 1000, 620)
				tt.SetTitleBar(ui.TitleBar{Height: height, Right: 138})
				header := requiredRect(t, tt, tr("设置顶栏"))
				if header.Y != max(float32(0), height-24) {
					t.Fatalf("header should move up by up to 24 DIPs: got %v, want %v", header.Y, max(float32(0), height-24))
				}
				button := requiredRect(t, tt, tr("返回播放"))
				if button.X+button.W > float32(1000)-138 || button.X+button.W != header.X+header.W-138 {
					t.Fatalf("return action overlaps caption controls or lost alignment: %+v", button)
				}
				for _, label := range []string{"设置标题图标", "设置标题", "设置说明", "返回播放"} {
					contained(t, label, header, requiredRect(t, tt, tr(label)))
				}
				content := requiredRect(t, tt, tr("设置内容区"))
				if content.Y < header.Y+header.H {
					t.Fatal("settings content overlaps the compact header")
				}
			})
		}
	}
}
