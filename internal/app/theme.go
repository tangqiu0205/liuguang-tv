package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var lightAppTheme = func() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f5f5f7")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#eaeaef")
	t.SurfacePressed = ui.Hex("#dddde5")
	t.Border = ui.Hex("#e3e3e8")
	t.Text = ui.Hex("#1d1d1f")
	t.TextMuted = ui.Hex("#68686e")
	// MyGo uses inverse colors for tooltips and toasts by default. Keep
	// transient hints on the same light surface as the rest of the UI.
	t.Inverse, t.InverseText = t.Surface, t.Text
	t.Accent = ui.Hex("#0065c8")
	t.AccentHover = ui.Hex("#0057ad")
	t.AccentPressed = ui.Hex("#00478f")
	t.AccentText = ui.Hex("#ffffff")
	t.Selection = ui.Hex("#dbeaff")
	t.Focus = t.Accent.Alpha(.5)
	t.Scrollbar = ui.Hex("#b8b8c0")
	t.Radius, t.FontSize, t.Spacing = 10, 13, 4
	return t
}()

type themePalette struct {
	Panel, Selected, Card, ActiveIcon, Success ui.Color
}

var nightPalette = themePalette{
	Panel: panelColor, Selected: selectedColor, Card: ui.Hex("#242427"), ActiveIcon: ui.Hex("#263e5b"),
	Success: ui.Hex("#70c6a0"),
}
var dayPalette = themePalette{
	Panel: ui.Hex("#f4f4f6"), Selected: ui.Hex("#e1edfc"), Card: ui.Hex("#ffffff"), ActiveIcon: ui.Hex("#d2e5fc"),
	Success: ui.Hex("#246e40"),
}

func paletteFor(t *ui.Theme) themePalette {
	if t.Dark {
		return nightPalette
	}
	return dayPalette
}

// Only the sidebar exposes the native material. Playback and settings surfaces
// remain opaque so that native video children never reveal the desktop.
func sidebarBackground(c *ui.Context) ui.Color {
	if c.Vibrancy() {
		return ui.Transparent
	}
	return panelBackground(c.Theme())
}

func panelBackground(t *ui.Theme) ui.Color    { return paletteFor(t).Panel }
func selectedBackground(t *ui.Theme) ui.Color { return paletteFor(t).Selected }
func selectionBorder(t *ui.Theme, selected bool) ui.Color {
	if selected {
		return t.Accent.Alpha(.35)
	}
	return ui.Transparent
}
func normalizedThemeMode(mode string) string {
	switch mode {
	case "light", "dark":
		return mode
	default:
		return "system"
	}
}
func nativeThemeSource(mode string) mygo.ThemeSource {
	switch normalizedThemeMode(mode) {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}
func themeForMode(mode string, systemDark bool) *ui.Theme {
	switch normalizedThemeMode(mode) {
	case "light":
		return lightAppTheme
	case "dark":
		return appTheme
	}
	if systemDark {
		return appTheme
	}
	return lightAppTheme
}
func (a *application) setThemeMode(mode string) {
	mode = normalizedThemeMode(mode)
	if normalizedThemeMode(a.state.ThemeMode) == mode {
		return
	}
	a.state.ThemeMode = mode
	// The native source also controls caption buttons and OS popup menus. Avoid
	// global app calls in headless UI tests; their system appearance comes from Context.
	if a.window != nil {
		mygo.Theme.SetSource(nativeThemeSource(mode))
	}
	a.status = tr("主题已切换")
	if a.path != "" {
		a.status = tr("主题设置已提交保存")
		a.pendingSettingsSuccess = tr("主题设置已保存")
		a.save()
	}
}
