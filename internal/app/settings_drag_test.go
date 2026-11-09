package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsDragAreaCoversTopPaddingWithoutBlockingButtons(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en"} {
		for _, width := range []int{1000, 1280} {
			t.Run(fmt.Sprintf("%s/%d", tag, width), func(t *testing.T) {
				useTestLanguage(t, tag)
				a := &application{showSettings: true}
				tt := ui.NewTester(a.view, width, 820)
				tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
				area := requiredRect(t, tt, tr("设置拖动区"))
				sidebar := requiredRect(t, tt, tr("设置标签栏"))
				if area.Y != 0 || area.X != sidebar.X+sidebar.W || area.X+area.W != float32(width) {
					t.Fatalf("drag area must cover the entire padded header: %+v", area)
				}
				contained(t, "settings heading", area, requiredRect(t, tt, tr("设置顶栏")))
				contained(t, "return button", area, requiredRect(t, tt, tr("返回播放")))
				if err := tt.Click(tr("播放器")); err != nil {
					t.Fatal(err)
				}
				if !a.settingsPlayerTab {
					t.Fatal("sidebar drag region intercepted a navigation click")
				}
				if err := tt.Click(tr("返回播放")); err != nil {
					t.Fatal(err)
				}
				if a.showSettings {
					t.Fatal("header drag region intercepted the return button")
				}
			})
		}
	}
}
