package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTooltipsAndToastsUseThemeSurface(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			theme := themeForMode(mode, false)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				ui.Column(c).Padding(40).Children(func() {
					if ui.Button(c, "Save").Tooltip("Save the note").Clicked() {
						c.Toast("Saved")
					}
				})
			}, 400, 300)
			assertFill := func(text string) {
				t.Helper()
				r := requiredRect(t, tt, text)
				pixel := tt.Image().RGBAAt(int(r.X)-3, int(r.Y+r.H/2))
				if pixel.R != theme.Surface.R || pixel.G != theme.Surface.G || pixel.B != theme.Surface.B {
					t.Fatalf("%q background does not follow %s theme: %+v, want %+v", text, mode, pixel, theme.Surface)
				}
			}
			button := requiredRect(t, tt, "Save")
			tt.Move(button.X+button.W/2, button.Y+button.H/2)
			tt.Advance(time.Second)
			assertFill("Save the note")
			if err := tt.Click("Save"); err != nil {
				t.Fatal(err)
			}
			tt.Advance(time.Second)
			assertFill("Saved")
			light, dark := luminance(theme.Inverse), luminance(theme.InverseText)
			if light < dark {
				light, dark = dark, light
			}
			if contrast := (light + .05) / (dark + .05); contrast < 4.5 {
				t.Fatalf("tooltip text contrast is too low: %.2f", contrast)
			}
		})
	}
}
