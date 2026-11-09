package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestStandalonePositioningPlatforms(t *testing.T) {
	for _, tc := range []struct {
		goos, session, wayland, backend string
		want                            bool
	}{
		{"windows", "", "", "", false},
		{"darwin", "", "", "", true},
		{"linux", "x11", "", "", true},
		{"linux", "", "", "", true},
		{"linux", "wayland", "", "", false},
		{"linux", "x11", "wayland-0", "", false},
		{"linux", "wayland", "wayland-0", "x11", false},
		{"linux", "", "", "wayland", false},
		{"freebsd", "", "", "", false},
	} {
		if got := standalonePositioningSupported(tc.goos, tc.session, tc.wayland, tc.backend); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}

func TestPlacementUsesCurrentContentOriginAndViewport(t *testing.T) {
	viewport := ui.Rect{X: 310, Y: 56, W: 800, H: 450}
	displays := []mygo.Display{{Bounds: mygo.Rectangle{Width: 1920, Height: 1080}, ScaleFactor: 1}}
	content := mygo.Rectangle{X: 100, Y: 80, Width: 1200, Height: 700}
	p := placementForViewport(content, viewport, displays, "linux")
	if p == nil || p.X != 410 || p.Y != 136 || p.Width != 800 || p.Height != 450 {
		t.Fatalf("placement: %+v", p)
	}
	content.X += 60 // Move without another viewport layout.
	next := placementForViewport(content, viewport, displays, "linux")
	if next.X != 470 || p.X != 410 {
		t.Fatalf("expected fresh independent snapshot: old=%+v new=%+v", p, next)
	}
}

func TestPlacementHiDPIAndNegativeMonitorOrigin(t *testing.T) {
	displays := []mygo.Display{
		{ID: 1, Bounds: mygo.Rectangle{Width: 1920, Height: 1080}, ScaleFactor: 1},
		{ID: 0, Label: "External", Bounds: mygo.Rectangle{X: -1600, Y: -200, Width: 1600, Height: 1000}, ScaleFactor: 2},
	}
	content := mygo.Rectangle{X: -1500, Y: -100, Width: 1200, Height: 700}
	viewport := ui.Rect{X: 310, Y: 56, W: 800, H: 450}
	for _, goos := range []string{"linux", "darwin"} {
		p := placementForViewport(content, viewport, displays, goos)
		if p == nil || p.X != 820 || p.Y != 312 || p.Width != 1600 || p.Height != 900 || p.ScreenName != "External" {
			t.Fatalf("%s: %+v", goos, p)
		}
		if goos == "linux" && p.Screen != 0 {
			t.Fatalf("must retain original GDK monitor ID: %+v", p)
		}
	}
}

func TestPlacementFallbackAndClipping(t *testing.T) {
	content := mygo.Rectangle{Width: 1200, Height: 700}
	displays := []mygo.Display{{Bounds: mygo.Rectangle{Width: 1000, Height: 800}}}
	for _, viewport := range []ui.Rect{{}, {W: -1, H: 10}, {X: 2000, W: 10, H: 10}, {X: float32(math.NaN()), W: 10, H: 10}} {
		if p := placementForViewport(content, viewport, displays, "linux"); p != nil {
			t.Fatalf("expected fallback for %+v: %+v", viewport, p)
		}
	}
	viewport := ui.Rect{X: -10, Y: 700, W: 200, H: 200}
	p := placementForViewport(content, viewport, displays, "linux")
	if p == nil || p.X != 0 || p.Y != 700 || p.Width != 190 || p.Height != 100 {
		t.Fatalf("clipped placement: %+v", p)
	}
	if placementForViewport(content, viewport, displays, "windows") != nil {
		t.Fatal("Windows must not get standalone geometry")
	}
	if placementForViewport(content, viewport, nil, "linux") != nil {
		t.Fatal("missing displays must fall back")
	}
	// No window handle or native API access is needed on Windows.
	if p := (&application{}).standalonePlacement(); p != nil {
		t.Fatalf("unexpected placement without a window: %+v", p)
	}
}
