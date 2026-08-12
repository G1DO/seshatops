package api

import (
	"sync"

	"github.com/G1DO/seshatops/platform"
)

// subscriberBuf is large enough for the M1 demo fanout without blocking the
// consumer. Overflow drops the update; clients must REST catch-up.
const subscriberBuf = 16

// Hub fans out post-commit AppliedUpdate notifications to SSE subscribers.
// NotifyApplied never blocks the caller.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan platform.AppliedUpdate]struct{}
}

// NewHub constructs an empty notification hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[chan platform.AppliedUpdate]struct{})}
}

// Subscribe registers a tenant-scoped update channel. The returned cancel
// function removes the subscriber without closing the channel, so a concurrent
// NotifyApplied cannot panic on send-to-closed-channel. SSE handlers exit via
// request context cancel rather than channel close.
func (h *Hub) Subscribe(tenantID string) (updates <-chan platform.AppliedUpdate, cancel func()) {
	ch := make(chan platform.AppliedUpdate, subscriberBuf)
	h.mu.Lock()
	if h.subs[tenantID] == nil {
		h.subs[tenantID] = make(map[chan platform.AppliedUpdate]struct{})
	}
	h.subs[tenantID][ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel = func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			set := h.subs[tenantID]
			if set == nil {
				return
			}
			delete(set, ch)
			if len(set) == 0 {
				delete(h.subs, tenantID)
			}
		})
	}
	return ch, cancel
}

// NotifyApplied implements platform.AppliedNotifier. Slow or full subscriber
// buffers drop the update rather than blocking projection commits. Sends run
// under the hub lock with a non-blocking select so unsubscribe cannot race a
// send onto a removed/closed channel.
func (h *Hub) NotifyApplied(update platform.AppliedUpdate) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[update.TenantID] {
		select {
		case ch <- update:
		default:
			// Drop; reconnect/REST catch-up is the recovery path.
		}
	}
}
