package webplayer

import "testing"

func TestSupportsURL(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{"http://127.0.0.1/live", true},
		{"https://example.org/live.m3u8?token=private", true},
		{"HTTP://example.org/live", true},
		{"rtsp://example.org/live", false},
		{"udp://239.1.1.1:1234", false},
		{"rtp://239.1.1.1:1234", false},
		{"file:///tmp/video.mp4", false},
		{"http://user:secret@example.org/live", false},
		{"/relative.m3u8", false},
		{"http:///missing-host", false},
		{"http://[invalid", false},
		{"", false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := SupportsURL(tc.raw); got != tc.want {
				t.Fatalf("SupportsURL=%v, want %v", got, tc.want)
			}
		})
	}
}
