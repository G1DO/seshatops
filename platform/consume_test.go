package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/relay"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func startRedpanda(t *testing.T) (seedBroker string, terminate func()) {
	t.Helper()
	ctx := context.Background()
	ctr, err := redpanda.Run(ctx, relay.RedpandaImage)
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
	resp, err := kadm.NewClient(admin).CreateTopics(ctx, 1, 1, nil, relay.Topic)
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

func TestRedpandaFirstDeliveryAndDuplicate(t *testing.T) {
	db := openTestDB(t)
	seed, stop := startRedpanda(t)
	t.Cleanup(stop)

	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	pub, err := relay.NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)
	drain, err := relay.DrainOnce(ctx, db, pub, "relay-owner", relay.DefaultLeaseTTL, 10)
	if err != nil || drain.Published != 1 {
		t.Fatalf("drain = %+v err=%v", drain, err)
	}

	cons, err := NewConsumer(db, seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cons.Close)

	deadline := time.Now().Add(30 * time.Second)
	var applied bool
	for time.Now().Before(deadline) && !applied {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cres, err := cons.ConsumeOnce(cctx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if cres.Acked > 0 {
			if !cres.LagKnown || cres.ObservedLag < 1 {
				t.Fatalf("consume lag signal = %+v", cres)
			}
			applied = true
		}
	}
	if !applied {
		t.Fatal("timeout waiting for first consume ack")
	}
	qty, ver, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}

	// Republish identical bytes (at-least-once duplicate).
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	if err := pub.Publish(ctx, relay.Topic, key, raw); err != nil {
		t.Fatal(err)
	}

	dupAck := false
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && !dupAck {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cres, err := cons.ConsumeOnce(cctx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if cres.Acked > 0 {
			dupAck = true
		}
	}
	if !dupAck {
		t.Fatal("timeout waiting for duplicate consume ack")
	}
	disp, _, ok, err := InboxDisposition(ctx, db, fx.Event.EventID)
	if err != nil || !ok || disp != DispositionDuplicateNoop {
		t.Fatalf("inbox after duplicate disp=%q ok=%v err=%v", disp, ok, err)
	}
	qty, ver, ok, err = ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection after duplicate qty=%d ver=%d", qty, ver)
	}
}

func TestCrashBeforeCommitThenRecover(t *testing.T) {
	// FC-012 / M1-INV-05: crash before PostgreSQL commit leaves no partial
	// inbox/projection effect; redelivery applies safely.
	db := openTestDB(t)
	fx := mustFixture(t)
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	pos := SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 42}

	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("crash before commit")
	})
	_, err := ProcessRecord(context.Background(), db, key, raw, pos)
	if err == nil {
		t.Fatal("expected crash error")
	}
	setTestFailBeforeCommitForTest(nil)

	var inboxN, projN int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inbox`).Scan(&inboxN); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inventory_projection`).Scan(&projN); err != nil {
		t.Fatal(err)
	}
	if inboxN != 0 || projN != 0 {
		t.Fatalf("pre-commit crash left partial state inbox=%d projection=%d", inboxN, projN)
	}

	res, err := ProcessRecord(context.Background(), db, key, raw, pos)
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionApplied || !res.ShouldAck {
		t.Fatalf("recover result = %+v", res)
	}
}

func TestCrashAfterCommitBeforeAckIsDuplicateNoop(t *testing.T) {
	// FC-012 / M1-INV-06: crash after DB commit and before broker ack may
	// redeliver; redelivery is a durable no-op business effect.
	db := openTestDB(t)
	seed, stop := startRedpanda(t)
	t.Cleanup(stop)

	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	pub, err := relay.NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)
	if _, err := relay.DrainOnce(ctx, db, pub, "relay-owner", relay.DefaultLeaseTTL, 10); err != nil {
		t.Fatal(err)
	}

	cons, err := NewConsumer(db, seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cons.Close)

	setTestSkipOffsetCommitForTest(func() error {
		return errors.New("skip offset commit")
	})
	t.Cleanup(func() { setTestSkipOffsetCommitForTest(nil) })

	deadline := time.Now().Add(30 * time.Second)
	var sawSkip bool
	for time.Now().Before(deadline) && !sawSkip {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cres, err := cons.ConsumeOnce(cctx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if cres.Processed > 0 && cres.Acked == 0 {
			if cres.AckWithheld == 0 {
				t.Fatalf("consume ack signal = %+v", cres)
			}
			sawSkip = true
		}
	}
	if !sawSkip {
		t.Fatal("expected processed-without-ack crash window")
	}
	qty, ver, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection after DB commit qty=%d ver=%d ok=%v", qty, ver, ok)
	}

	setTestSkipOffsetCommitForTest(nil)
	// Force redelivery by publishing again (simulates restart with uncommitted offset).
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	if err := pub.Publish(ctx, relay.Topic, key, raw); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(30 * time.Second)
	var dup bool
	for time.Now().Before(deadline) && !dup {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cres, err := cons.ConsumeOnce(cctx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if cres.Acked > 0 {
			dup = true
		}
	}
	if !dup {
		t.Fatal("timeout waiting for redelivery ack")
	}
	disp, _, ok, err := InboxDisposition(ctx, db, fx.Event.EventID)
	if err != nil || !ok || disp != DispositionDuplicateNoop {
		t.Fatalf("expected duplicate_noop, got disp=%q ok=%v err=%v", disp, ok, err)
	}
}
