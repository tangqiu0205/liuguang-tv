package player

import "testing"

func TestPlaybackObservation(t *testing.T) {
	for _, message := range []string{
		`{}`, `{"event":"property-change","name":"time-pos","data":null}`,
		`{"event":"property-change","name":"time-pos","data":-1}`, `invalid`,
		`{"event":"property-change","name":"time-pos","data":"bad"}`,
		`{"event":"property-change","name":"pause","data":null}`,
		`{"event":"property-change","name":"pause","data":1}`,
		`{"event":"property-change","name":"other","data":true}`,
	} {
		if _, ok := playbackStatus([]byte(message)); ok {
			t.Fatalf("false observation: %s", message)
		}
	}
	for _, message := range []string{`{"event":"property-change","name":"time-pos","data":0}`, `{"event":"property-change","name":"time-pos","data":2.5}`} {
		s, ok := playbackStatus([]byte(message))
		if !ok || s.Position == nil {
			t.Fatal(message)
		}
	}
	s, ok := playbackStatus([]byte(`{"event":"property-change","name":"pause","data":false}`))
	if !ok || s.Paused == nil || *s.Paused {
		t.Fatal("false pause is a valid observation")
	}
	s, ok = playbackStatus([]byte(`{"event":"property-change","name":"paused-for-cache","data":true}`))
	if !ok || s.Buffering == nil || !*s.Buffering {
		t.Fatal("buffering was not observed")
	}
}
