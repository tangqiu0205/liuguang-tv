package loadqueue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeadlineStartsAfterQueueWait(t *testing.T) {
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	parent, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		ctx, release, err := Acquire(parent, slots, 30*time.Millisecond)
		if err != nil {
			result <- err
			return
		}
		// Waiting longer than the request timeout must not expire the request.
		value := ctx.Err()
		release()
		result <- value
	}()
	time.Sleep(80 * time.Millisecond)
	<-slots
	if err := <-result; err != nil {
		t.Fatalf("queue consumed request deadline: %v", err)
	}
	if len(slots) != 0 {
		t.Fatal("slot was not released")
	}
}
func TestQueuedAndPreCancelledJobsDoNotLeakSlots(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		slots := make(chan struct{}, 1)
		if occupied {
			slots <- struct{}{}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := Acquire(ctx, slots, time.Second); !errors.Is(err, context.Canceled) {
			t.Fatalf("ignored cancellation: %v", err)
		}
		expected := 0
		if occupied {
			expected = 1
		}
		if len(slots) != expected {
			t.Fatal("cancelled job leaked or released somebody else's slot")
		}
	}
}
func TestDeadlineAndReleaseCancelContext(t *testing.T) {
	slots := make(chan struct{}, 1)
	ctx, release, err := Acquire(context.Background(), slots, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatal(ctx.Err())
	}
	release()
	ctx, release, err = Acquire(context.Background(), slots, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if !errors.Is(ctx.Err(), context.Canceled) || len(slots) != 0 {
		t.Fatal("release did not cancel and free slot")
	}
}
