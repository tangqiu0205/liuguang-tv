package player

import (
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"strings"
	"testing"
)

func TestSafeRequestHeaderArguments(t *testing.T) {
	ch := playlist.Channel{Name: "Demo", URL: "https://example.org/live", Headers: playlist.StreamHeaders{UserAgent: "Test --script=evil", Referer: "https://example.org/watch?a=1&b=2"}}
	args := arguments(ch, Options{}, "")
	foundUA, foundRef := false, false
	for _, arg := range args {
		if arg == "--script=evil" {
			t.Fatal("header escaped into player option")
		}
		foundUA = foundUA || arg == "--user-agent="+ch.Headers.UserAgent
		foundRef = foundRef || arg == "--referrer="+ch.Headers.Referer
	}
	if !foundUA || !foundRef || args[len(args)-2] != "--" {
		t.Fatal(args)
	}
	ch.Headers.UserAgent = "secret\r\nInjected: yes"
	if _, err := (&Player{}).Play("does-not-exist", ch); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe header accepted/leaked: %v", err)
	}
	plain := strings.Join(arguments(playlist.Channel{URL: ch.URL}, Options{}, ""), "\n")
	if strings.Contains(plain, "--user-agent=") || strings.Contains(plain, "--referrer=") {
		t.Fatal("headers leaked across sessions")
	}
}
