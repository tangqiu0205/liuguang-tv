package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/assets"
)

var (
	appTheme = func() *ui.Theme {
		t := ui.DarkTheme()
		t.Background = ui.Hex("#141416")
		t.Surface = ui.Hex("#242427")
		t.SurfaceHover = ui.Hex("#303034")
		t.SurfacePressed = ui.Hex("#3a3a40")
		t.Border = ui.Hex("#303034")
		t.Text = ui.Hex("#f5f5f7")
		t.TextMuted = ui.Hex("#a1a1aa")
		t.Inverse, t.InverseText = t.Surface, t.Text
		t.Accent = ui.Hex("#78b5ff")
		t.AccentHover = ui.Hex("#9ac8ff")
		t.AccentPressed = ui.Hex("#5ca3f5")
		t.AccentText = ui.Hex("#101c2d")
		t.Selection = ui.Hex("#263e5b")
		t.Focus = t.Accent.Alpha(.45)
		t.Scrollbar = ui.Hex("#515158")
		t.Radius = 10
		t.FontSize = 13
		t.Spacing = 4
		return t
	}()
	panelColor    = ui.Hex("#1c1c1f")
	selectedColor = ui.Hex("#21344d")
)

func svgIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	appIcon             = ui.MustParseSVG(assets.IconSVG)
	searchIcon          = svgIcon(`<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>`)
	closeIcon           = svgIcon(`<path d="m6 6 12 12M18 6 6 18"/>`)
	addIcon             = svgIcon(`<path d="M12 4v16M4 12h16"/>`)
	leftIcon            = svgIcon(`<path d="m14 6-6 6 6 6"/>`)
	rightIcon           = svgIcon(`<path d="m10 6 6 6-6 6"/>`)
	downIcon            = svgIcon(`<path d="m6 9 6 6 6-6"/>`)
	updateIcon          = svgIcon(`<path d="M12 16V3m-5 5 5-5 5 5M4 15v5a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-5"/>`)
	refreshIcon         = svgIcon(`<path d="M20 7v5h-5M4 17v-5h5M6 7a7 7 0 0 1 12-1l2 6M4 12l2 6a7 7 0 0 0 12-1"/>`)
	deleteIcon          = svgIcon(`<path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v5M14 11v5"/>`)
	tvIcon              = svgIcon(`<rect x="3" y="5" width="18" height="14" rx="3"/><path d="m9 2 3 3 3-3M8 22h8"/>`)
	playIcon            = svgIcon(`<path d="m9 5 11 7-11 7Z"/>`)
	pauseIcon           = svgIcon(`<path d="M8 5v14M16 5v14"/>`)
	previousChannelIcon = svgIcon(`<path d="M5 5v14m14-14L8 12l11 7Z"/>`)
	nextChannelIcon     = svgIcon(`<path d="M19 5v14M5 5l11 7-11 7Z"/>`)
	stopIcon            = svgIcon(`<rect x="6" y="6" width="12" height="12" rx="2"/>`)
	starIcon            = svgIcon(`<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z"/>`)
	windowExpandIcon    = svgIcon(`<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 8h18m-8 4h5v5m0-5-7 7"/>`)
	expandIcon          = svgIcon(`<path d="M8 3H3v5m13-5h5v5M3 16v5h5m13-5v5h-5"/>`)
	backupIcon          = svgIcon(`<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 4 16 4 16 0V5M4 12c0 4 16 4 16 0"/>`)
	infoIcon            = svgIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>`)
	settingsIcon        = svgIcon(`<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="9" cy="6" r="2"/><circle cx="15" cy="12" r="2"/><circle cx="9" cy="18" r="2"/>`)
	soundIcon           = svgIcon(`<path d="M4 9h4l5-4v14l-5-4H4Zm12-1a6 6 0 0 1 0 8m3-11a10 10 0 0 1 0 14"/>`)
	muteIcon            = svgIcon(`<path d="M4 9h4l5-4v14l-5-4H4Zm13 0 5 6m0-6-5 6"/>`)
)

func iconButton(c *ui.Context, label string, icon *ui.SVG, active, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(34, 34).Radius(12).Disabled(disabled)
	color, bg := t.TextMuted, ui.Transparent
	if active {
		color = t.Accent
		bg = selectedBackground(t)
	}
	b.Background(bg)
	if !disabled && b.Hovered() {
		if !active {
			color = t.Text
		}
		b.Background(t.SurfaceHover)
	}
	if !disabled && b.Pressed() {
		b.Scale(.92, .92)
	}
	border := selectionBorder(t, active)
	if disabled {
		color = color.Alpha(.35)
		border = border.Alpha(.35)
	}
	return b.Border(1, border).Children(func() { ui.Icon(c, icon).Size(18, 18).TextColor(color) })
}

func badge(c *ui.Context, text string, active bool) {
	color, bg := c.Theme().TextMuted, c.Theme().Surface
	if active {
		color = c.Theme().Accent
		bg = selectedBackground(c.Theme())
	}
	ui.Text(c, text).FontSize(10).Bold().TextColor(color).Background(bg).Border(1, selectionBorder(c.Theme(), active)).Radius(5).Padding(4, 7).Shrink(0)
}
