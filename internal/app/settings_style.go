package app

import "github.com/egoist/mygo/ui"

var (
	sourceLayersIcon = svgIcon(`<path d="m12 3 9 5-9 5-9-5 9-5Zm-9 9 9 5 9-5M3 16l9 5 9-5"/>`)
	calendarIcon     = svgIcon(`<rect x="3" y="5" width="18" height="16" rx="3"/><path d="M7 3v4M17 3v4M3 10h18"/>`)
	linkIcon         = svgIcon(`<path d="m10 13 4-4m-6 7-1 1a4 4 0 0 1-6-6l4-4a4 4 0 0 1 6 0m2 1 1-1a4 4 0 0 1 6 6l-4 4a4 4 0 0 1-6 0"/>`)
	editIcon         = svgIcon(`<path d="m15 4 5 5M4 20l5-1L21 7a2 2 0 0 0-4-4L5 15l-1 5Z"/>`)
	gearIcon         = svgIcon(`<path d="m10 3-.7 2.3-2 .9-2.2-.6-2 3.4 1.6 1.7v2.6L3.1 15l2 3.4 2.2-.6 2 .9.7 2.3h4l.7-2.3 2-.9 2.2.6 2-3.4-1.6-1.7v-2.6L20.9 9l-2-3.4-2.2.6-2-.9L14 3h-4Z"/><circle cx="12" cy="12" r="3"/>`)
)

func settingsAction(c *ui.Context, label, title string, icon *ui.SVG, primary, disabled bool) ui.Element {
	t := c.Theme()
	bg, color, border := t.Surface, t.Text, t.Border
	if primary {
		bg, color, border = t.Accent, t.AccentText, t.Accent
	}
	b := ui.ButtonBase(c).Label(label).Disabled(disabled).Height(34).Padding(0, 14).Gap(7).Radius(8)
	if !disabled && b.Hovered() {
		if primary {
			bg = t.AccentHover
		} else {
			bg = t.SurfaceHover
		}
	}
	if disabled {
		color = color.Alpha(.4)
	}
	return b.Background(bg).Border(1, border).Shadow(0, 2, 5, -2, t.Accent.Alpha(.14)).Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(color)
		ui.Text(c, title).FontSize(12).TextColor(color)
	})
}
