// Package statework serializes catalog construction and durable configuration
// writes away from the UI thread. Only the latest pending snapshot is retained.
package statework

import (
	"sync"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

type Request struct {
	Sequence uint64
	State    store.State
	Build    bool
}
type Result struct {
	Request
	Err error
}
type Worker struct {
	mu        sync.Mutex
	pending   *Request
	latest    uint64
	closing   bool
	lastError error
	wake      chan struct{}
	done      chan struct{}
	path      string
	notify    func(Result)
}

func New(path string, notify func(Result)) *Worker {
	w := &Worker{path: path, notify: notify, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go w.run()
	return w
}
func (w *Worker) Submit(request Request) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing || request.Sequence <= w.latest {
		return false
	}
	w.latest = request.Sequence
	copy := request
	w.pending = &copy
	select {
	case w.wake <- struct{}{}:
	default:
	}
	return true
}

// Close stops admission and flushes the latest snapshot. It does not require
// the UI to execute a completion callback, so closing a window cannot deadlock.
func (w *Worker) Close() error {
	w.mu.Lock()
	w.closing = true
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
	<-w.done
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastError
}
func (w *Worker) run() {
	defer close(w.done)
	for {
		<-w.wake
		// Batch bursts of refresh/favorite changes without a long quiet-time wait.
		w.mu.Lock()
		closing := w.closing
		w.mu.Unlock()
		if !closing {
			time.Sleep(40 * time.Millisecond)
		}
		for {
			w.mu.Lock()
			request := w.pending
			w.pending = nil
			closing = w.closing
			w.mu.Unlock()
			if request == nil {
				if closing {
					return
				}
				break
			}
			if request.Build {
				request.State.RebuildChannels()
			}
			w.mu.Lock()
			superseded := w.latest > request.Sequence
			w.mu.Unlock()
			if superseded {
				continue
			}
			err := store.Save(w.path, request.State)
			w.mu.Lock()
			w.lastError = err
			w.mu.Unlock()
			if w.notify != nil {
				w.notify(Result{Request: *request, Err: err})
			}
			// The next pending request (if any) is processed by this same writer;
			// no older write can race a newer one and overwrite the latest state.
		}
	}
}
