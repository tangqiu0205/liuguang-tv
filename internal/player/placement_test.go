package player

import (
	"strings"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestStandalonePlacementArguments(t *testing.T) {
	p := WindowPlacement{X: 310, Y: 56, Width: 960, Height: 540, Screen: 1, ScreenName: "Built-in Retina Display"}
	for _, goos := range []string{"linux", "darwin"} {
		args := strings.Join(p.arguments(goos), "\n")
		for _, want := range []string{"--geometry=960x540+310+56", "--border=no", "--keepaspect-window=no", "--keepaspect=yes", "--hidpi-window-scale=no"} {
			if !strings.Contains(args, want) {
				t.Errorf("%s: missing %s in %s", goos, want, args)
			}
		}
		if strings.Contains(args, "--ontop") || strings.Contains(args, "--osc=no") {
			t.Fatalf("standalone player must keep normal stacking and controls: %s", args)
		}
		if goos == "darwin" {
			if !strings.Contains(args, "--screen-name=Built-in Retina Display") || !strings.Contains(args, "--macos-geometry-calculation=whole") {
				t.Fatal(args)
			}
		} else if !strings.Contains(args, "--screen=1") || strings.Contains(args, "--macos-") {
			t.Fatal(args)
		}
	}
	for _, goos := range []string{"windows", "freebsd"} {
		if args := p.arguments(goos); len(args) != 0 {
			t.Fatalf("must not alter %s: %v", goos, args)
		}
	}
}

func TestInvalidPlacementDoesNotChangeArguments(t *testing.T) {
	for _, p := range []WindowPlacement{{}, {Width: -1, Height: 10}, {Width: 10, Height: 0}, {X: -1, Width: 10, Height: 10}, {Y: -1, Width: 10, Height: 10}, {Screen: -1, Width: 10, Height: 10}} {
		if args := p.arguments("linux"); len(args) != 0 {
			t.Fatalf("invalid placement accepted: %+v: %v", p, args)
		}
	}
}

func TestEmbeddedPlaybackIgnoresStandalonePlacement(t *testing.T) {
	ch := playlist.Channel{Name: "A", URL: "https://example.org/live"}
	opt := Options{Window: 123, Controls: Controls{Volume: 70}}
	before := strings.Join(arguments(ch, opt, "pipe"), "\n")
	opt.Placement = &WindowPlacement{Width: 800, Height: 600}
	after := strings.Join(arguments(ch, opt, "pipe"), "\n")
	if before != after {
		t.Fatalf("embedded arguments changed:\n%s\n%s", before, after)
	}
}
