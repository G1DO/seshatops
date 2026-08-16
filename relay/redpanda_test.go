package relay

import (
	"context"
	"testing"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func startRedpanda(t *testing.T) (seedBroker string, terminate func()) {
	t.Helper()
	ctx := context.Background()
	ctr, err := redpanda.Run(ctx, RedpandaImage)
	if err != nil {
		t.Skipf("Redpanda integration tests require Docker: %v", err)
	}
	terminate = func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminate redpanda: %v", err)
		}
	}
	seed, err := ctr.KafkaSeedBroker(ctx)
	if err != nil {
		terminate()
		t.Fatalf("kafka seed broker: %v", err)
	}
	admin, err := kgo.NewClient(kgo.SeedBrokers(seed))
	if err != nil {
		terminate()
		t.Fatalf("admin client: %v", err)
	}
	defer admin.Close()
	resp, err := kadm.NewClient(admin).CreateTopics(ctx, 1, 1, nil, Topic)
	if err != nil {
		terminate()
		t.Fatalf("create topic: %v", err)
	}
	if err := resp.Error(); err != nil {
		terminate()
		t.Fatalf("create topic response: %v", err)
	}
	return seed, terminate
}

func consumeAll(t *testing.T, seed string, want int, timeout time.Duration) []kgo.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seed),
		kgo.ConsumeTopics(Topic),
		kgo.ConsumerGroup("seshatops-m1-relay-test"),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	defer cl.Close()

	var out []kgo.Record
	for len(out) < want {
		fetches := cl.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				if fe.Err == context.DeadlineExceeded || fe.Err == context.Canceled {
					t.Fatalf("timeout waiting for %d records, got %d: %v", want, len(out), fe.Err)
				}
				t.Fatalf("fetch error: %v", fe.Err)
			}
		}
		fetches.EachRecord(func(r *kgo.Record) {
			out = append(out, *r)
		})
		if ctx.Err() != nil && len(out) < want {
			t.Fatalf("timeout waiting for %d records, got %d", want, len(out))
		}
	}
	return out[:want]
}

func TestRedpandaNormalPublication(t *testing.T) {
	db := openTestDB(t)
	seed, stop := startRedpanda(t)
	t.Cleanup(stop)

	fx, res := seedAndAccept(t, db)
	ctx := context.Background()
	pub, err := NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)

	out, err := DrainOnce(ctx, db, pub, "relay-1", DefaultLeaseTTL, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Published != 1 {
		t.Fatalf("drain %+v", out)
	}

	recs := consumeAll(t, seed, 1, 30*time.Second)
	wantKey := AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID)
	if string(recs[0].Key) != wantKey {
		t.Fatalf("key = %q want %q", recs[0].Key, wantKey)
	}
	if string(recs[0].Value) != string(res.EventBytes) {
		t.Fatal("redpanda value does not match stored outbox bytes")
	}
	parsed, err := event.Parse(recs[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := event.ContentHash(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if hash != res.ContentHash {
		t.Fatalf("hash = %s want %s", hash, res.ContentHash)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPublished {
		t.Fatalf("status = %s", status)
	}
}

func TestRedpandaBrokerOutagePersistenceAndRecovery(t *testing.T) {
	db := openTestDB(t)
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}

	// Source transaction commits while Redpanda is unavailable.
	res, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	badPub, err := NewFranzPublisher("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	out, err := DrainOnce(ctx, db, badPub, "relay-outage", DefaultLeaseTTL, 1)
	badPub.Close()
	if err != nil {
		t.Fatal(err)
	}
	if out.Transient != 1 {
		t.Fatalf("expected transient publish failure, got %+v", out)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPending {
		t.Fatalf("status = %s want pending", status)
	}
	b, err := InspectBacklog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if b.Pending != 1 {
		t.Fatalf("backlog should expose unpublished intent: %+v", b)
	}

	clearBackoff(t, db, res.EventID)

	seed, stop := startRedpanda(t)
	t.Cleanup(stop)
	pub, err := NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)

	out, err = DrainOnce(ctx, db, pub, "relay-recovery", DefaultLeaseTTL, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Published != 1 {
		t.Fatalf("recovery drain %+v", out)
	}
	recs := consumeAll(t, seed, 1, 30*time.Second)
	if string(recs[0].Value) != string(res.EventBytes) {
		t.Fatal("recovery publication rewrote event content")
	}
	parsed, err := event.Parse(recs[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.CheckIdentityConflict(parsed, fx.Event); err != nil {
		t.Fatal(err)
	}
}

func TestRedpandaLineageBrokerOutagePersistenceAndRecovery(t *testing.T) {
	db := openTestDB(t)
	fx, hops, orderRes := seedAndAcceptLineage(t, db)
	ctx := context.Background()

	wantBytes := map[string][]byte{
		hops[0].EventID:  hops[0].EventBytes,
		hops[1].EventID:  hops[1].EventBytes,
		hops[2].EventID:  hops[2].EventBytes,
		hops[3].EventID:  hops[3].EventBytes,
		orderRes.EventID: orderRes.EventBytes,
	}

	badPub, err := NewFranzPublisher("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	outageCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	out, err := DrainOnce(outageCtx, db, badPub, "relay-lineage-outage", DefaultLeaseTTL, 10)
	cancel()
	badPub.Close()
	if err != nil {
		t.Fatal(err)
	}
	if out.Claimed != 5 || out.Transient != 5 {
		t.Fatalf("expected five transient publish failures, got %+v", out)
	}
	assertFullLineageSourceRetained(t, db)
	for id := range wantBytes {
		status, _, _, _ := outboxStatus(t, db, id)
		if status != StatusPending {
			t.Fatalf("status = %s want pending for %s", status, id)
		}
	}

	clearAllPendingBackoff(t, db)

	seed, stop := startRedpanda(t)
	t.Cleanup(stop)
	pub, err := NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)

	out, err = DrainOnce(ctx, db, pub, "relay-lineage-recovery", DefaultLeaseTTL, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Published != 5 {
		t.Fatalf("recovery drain %+v", out)
	}
	recs := consumeAll(t, seed, 5, 30*time.Second)
	seen := map[string]bool{}
	for _, rec := range recs {
		parsed, err := event.Parse(rec.Value)
		if err != nil {
			t.Fatal(err)
		}
		stored, ok := wantBytes[parsed.EventID]
		if !ok {
			t.Fatalf("unexpected event_id %s", parsed.EventID)
		}
		if string(rec.Value) != string(stored) {
			t.Fatal("recovery publication rewrote event content")
		}
		if err := event.CheckIdentityConflict(parsed, lineageEnvelope(t, fx, parsed.EventID)); err != nil {
			t.Fatal(err)
		}
		seen[parsed.EventID] = true
	}
	if len(seen) != 5 {
		t.Fatalf("recovered identities = %d", len(seen))
	}
}

func TestRedpandaAmbiguousWindowDuplicate(t *testing.T) {
	db := openTestDB(t)
	seed, stop := startRedpanda(t)
	t.Cleanup(stop)
	fx, res := seedAndAccept(t, db)
	ctx := context.Background()
	pub, err := NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)

	claimed, err := ClaimDue(ctx, db, "relay-amb-1", DefaultLeaseTTL, 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %+v", err, claimed)
	}
	key := []byte(AggregateKey(claimed[0].TenantID, claimed[0].AggregateType, claimed[0].AggregateID))
	if err := pub.Publish(ctx, Topic, key, claimed[0].EventBytes); err != nil {
		t.Fatal(err)
	}
	// Crash window: broker accepted, source status not updated.
	expireLease(t, db, res.EventID)

	out, err := DrainOnce(ctx, db, pub, "relay-amb-2", DefaultLeaseTTL, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Published != 1 {
		t.Fatalf("drain %+v", out)
	}
	recs := consumeAll(t, seed, 2, 30*time.Second)
	if string(recs[0].Value) != string(recs[1].Value) || string(recs[0].Key) != string(recs[1].Key) {
		t.Fatal("duplicate publications must share identity and content")
	}
	for _, r := range recs {
		parsed, err := event.Parse(r.Value)
		if err != nil {
			t.Fatal(err)
		}
		if err := event.CheckIdentityConflict(parsed, fx.Event); err != nil {
			t.Fatal(err)
		}
	}
}
