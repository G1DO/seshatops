package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWorkerRetriesWithBoundedBackoffAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	health := make(chan bool, 4)
	done := make(chan struct{})
	go func() {
		runWorker(ctx, "test", time.Millisecond, time.Millisecond, 4*time.Millisecond, func(context.Context) error {
			if calls.Add(1) == 1 {
				return errors.New("temporary")
			}
			return nil
		}, func(healthy bool) { health <- healthy })
		close(done)
	}()

	select {
	case healthy := <-health:
		if healthy {
			t.Fatal("first failed cycle reported healthy")
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not retry failed cycle")
	}
	select {
	case healthy := <-health:
		if !healthy {
			t.Fatal("successful retry reported unhealthy")
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not report successful retry")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	if calls.Load() < 2 {
		t.Fatalf("worker calls = %d", calls.Load())
	}
}

func TestNextBackoffIsBounded(t *testing.T) {
	if got := nextBackoff(time.Millisecond, 4*time.Millisecond); got != 2*time.Millisecond {
		t.Fatalf("backoff = %s", got)
	}
	if got := nextBackoff(3*time.Millisecond, 4*time.Millisecond); got != 4*time.Millisecond {
		t.Fatalf("capped backoff = %s", got)
	}
	if got := nextBackoff(4*time.Millisecond, 4*time.Millisecond); got != 4*time.Millisecond {
		t.Fatalf("maximum backoff = %s", got)
	}
}
