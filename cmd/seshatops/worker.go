package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/G1DO/seshatops/observability"
)

func runWorker(
	ctx context.Context,
	name string,
	interval time.Duration,
	retryBase time.Duration,
	retryMax time.Duration,
	cycle func(context.Context) error,
	setHealthy func(bool),
) {
	if interval <= 0 {
		interval = time.Second
	}
	if retryBase <= 0 {
		retryBase = time.Second
	}
	if retryMax < retryBase {
		retryMax = retryBase
	}
	backoff := retryBase
	obsCtx := ctx
	if generated, _, err := observability.EnsureCorrelationID(ctx); err == nil {
		obsCtx = generated
	}
	for {
		err := cycle(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			setHealthy(false)
			observability.Log(obsCtx, slog.Default(), observability.EventWorkerRetrying, observability.Fields{Worker: observability.Worker(name)})
			if !waitFor(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, retryMax)
			continue
		}
		setHealthy(true)
		backoff = retryBase
		if !waitFor(ctx, interval) {
			return
		}
	}
}

func waitFor(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}
