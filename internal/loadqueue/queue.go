// Package loadqueue separates cancellable queue waiting from request timeouts.
package loadqueue

import (
	"context"
	"time"
)

// Acquire starts the request deadline only after a slot is available. The
// caller must call release exactly once; cancellation also applies while queued.
func Acquire(parent context.Context, slots chan struct{}, timeout time.Duration) (context.Context, func(), error) {
	select {
	case slots <- struct{}{}:
	case <-parent.Done():
		return nil, nil, parent.Err()
	}
	if err := parent.Err(); err != nil {
		<-slots
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, func() { cancel(); <-slots }, nil
}
