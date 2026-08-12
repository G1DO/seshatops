package api

import (
	"sync"
	"testing"
	"time"

	"github.com/G1DO/seshatops/platform"
)

func TestHubUnsubscribeDoesNotPanicUnderNotify(t *testing.T) {
	hub := NewHub()
	const tenant = "11111111-1111-4111-8111-111111111111"
	update := platform.AppliedUpdate{
		TenantID:         tenant,
		ItemID:           "item-flour-001",
		QuantityOnHand:   8,
		AggregateVersion: 1,
		EventID:          "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1",
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, cancel := hub.Subscribe(tenant)
			hub.NotifyApplied(update)
			cancel()
			hub.NotifyApplied(update)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out; likely deadlock or stuck notify")
	}
}
