package player

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestRejectUnsafeStream(t *testing.T) {
	p := &Player{}
	for _, raw := range []string{"--script=evil", "file:///etc/passwd", "javascript:evil"} {
		if _, err := p.Play("mpv", playlist.Channel{URL: raw}); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	p.Stop()
}
func TestEmbeddedArguments(t *testing.T) {
	args := arguments(playlist.Channel{Name: "A", URL: "https://example.org/live"}, Options{Window: 0xF1234567, Controls: Controls{Volume: 120}}, "test-pipe")
	joined := strings.Join(args, "\n")
	for _, want := range []string{"--wid=4045620583", "--volume=100", "--osc=no", "--input-default-bindings=no", "--input-vo-keyboard=no", "--input-ipc-server=test-pipe", "--\nhttps://example.org/live"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
}

func TestControlProtocol(t *testing.T) {
	var buf bytes.Buffer
	if err := writeControls(&buf, Controls{Volume: -10, Muted: true, Paused: true}); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&buf)
	for _, want := range []string{`{"command":["set_property","volume",0]}`, `{"command":["set_property","mute",true]}`, `{"command":["set_property","pause",true]}`} {
		var got json.RawMessage
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}
}

func TestMissingPlayer(t *testing.T) {
	if _, err := Resolve("this-player-does-not-exist-9340"); err == nil {
		t.Fatal("expected missing player error")
	}
}
