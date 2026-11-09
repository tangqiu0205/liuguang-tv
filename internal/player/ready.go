package player

import (
	"encoding/json"
	"io"
	"time"
)

// PlaybackStatus contains only observed properties; nil means unchanged.
type PlaybackStatus struct {
	Position  *float64
	Paused    *bool
	Buffering *bool
}

func playbackStatus(message []byte) (PlaybackStatus, bool) {
	var event struct {
		Event string          `json:"event"`
		Name  string          `json:"name"`
		Data  json.RawMessage `json:"data"`
	}
	if json.Unmarshal(message, &event) != nil || event.Event != "property-change" {
		return PlaybackStatus{}, false
	}
	var status PlaybackStatus
	switch event.Name {
	case "time-pos":
		if json.Unmarshal(event.Data, &status.Position) != nil || status.Position == nil || *status.Position < 0 {
			return status, false
		}
	case "pause":
		if json.Unmarshal(event.Data, &status.Paused) != nil || status.Paused == nil {
			return status, false
		}
	case "paused-for-cache":
		if json.Unmarshal(event.Data, &status.Buffering) != nil || status.Buffering == nil {
			return status, false
		}
	default:
		return status, false
	}
	return status, true
}
func (s *session) monitorPlayback(ready func(), progress func(PlaybackStatus)) {
	notified := false
	for {
		select {
		case <-s.stopped:
			return
		default:
		}
		f, err := openMonitorPipe(s.pipe)
		if err == nil {
			monitorDone := make(chan struct{})
			go func() {
				select {
				case <-s.stopped:
					_ = f.Close()
				case <-monitorDone:
				}
			}()
			enc := json.NewEncoder(f)
			for i, name := range []string{"pause", "paused-for-cache", "time-pos"} {
				err = enc.Encode(map[string]any{"command": []any{"observe_property", i + 1, name}})
				if err != nil {
					break
				}
			}
			decoder := json.NewDecoder(f)
			for err == nil {
				var message json.RawMessage
				err = decoder.Decode(&message)
				if err != nil {
					break
				}
				status, ok := playbackStatus(message)
				if !ok {
					continue
				}
				select {
				case <-s.stopped:
					err = io.EOF
					continue
				default:
				}
				if status.Position != nil && !notified {
					notified = true
					if ready != nil {
						ready()
					}
				}
				if progress != nil {
					progress(status)
				}
			}
			close(monitorDone)
			_ = f.Close()
		}
		select {
		case <-s.stopped:
			return
		case <-time.After(80 * time.Millisecond):
		}
	}
}
