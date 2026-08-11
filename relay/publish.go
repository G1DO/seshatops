package relay

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/G1DO/seshatops/event"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Publisher publishes one Kafka/Redpanda message. A nil error means the broker
// acknowledged the produce. Implementations must not invent exactly-once
// guarantees.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

// FranzPublisher publishes with franz-go using synchronous produce ACKs.
type FranzPublisher struct {
	client *kgo.Client
}

// NewFranzPublisher dials seed brokers for produce-only use.
func NewFranzPublisher(seeds ...string) (*FranzPublisher, error) {
	if len(seeds) == 0 {
		return nil, errors.New("relay: at least one broker seed required")
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.DisableIdempotentWrite(), // at-least-once; not Redpanda/Kafka EOS
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
		kgo.DialTimeout(3*time.Second),
		kgo.ConnIdleTimeout(5*time.Second),
		kgo.RequestRetries(1),
		kgo.ProduceRequestTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("relay: franz client: %w", err)
	}
	return &FranzPublisher{client: cl}, nil
}

// Close releases the underlying client.
func (p *FranzPublisher) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}

// Publish produces one record and waits for broker acknowledgement.
func (p *FranzPublisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	if p == nil || p.client == nil {
		return errors.New("relay: nil franz publisher")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	res := p.client.ProduceSync(ctx, &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	})
	if err := res.FirstErr(); err != nil {
		return fmt.Errorf("relay: produce: %w", err)
	}
	return nil
}

// DrainResult summarizes one DrainOnce cycle.
type DrainResult struct {
	Claimed     int
	Published   int
	Transient   int
	Quarantined int
	// Ambiguous counts rows where broker ACK succeeded but MarkPublished failed.
	// The row remains publishing until the lease expires for duplicate-safe retry.
	Ambiguous int
}

// DrainOnce claims up to limit due rows and attempts publication for each.
// MarkPublished runs only after Publisher returns nil (broker ACK).
func DrainOnce(ctx context.Context, db *sql.DB, pub Publisher, owner string, leaseTTL time.Duration, limit int) (DrainResult, error) {
	if pub == nil {
		return DrainResult{}, errors.New("relay: publisher required")
	}
	recs, err := ClaimDue(ctx, db, owner, leaseTTL, limit)
	if err != nil {
		return DrainResult{}, err
	}
	var out DrainResult
	out.Claimed = len(recs)
	for _, rec := range recs {
		err := publishOne(ctx, db, pub, owner, rec)
		var pe *publishOutcome
		if errors.As(err, &pe) {
			switch pe.kind {
			case outcomePublished:
				out.Published++
			case outcomeTransient:
				out.Transient++
			case outcomeQuarantine:
				out.Quarantined++
			case outcomeAmbiguous:
				out.Ambiguous++
			}
			continue
		}
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

type outcomeKind int

const (
	outcomePublished outcomeKind = iota
	outcomeTransient
	outcomeQuarantine
	outcomeAmbiguous
)

type publishOutcome struct {
	kind outcomeKind
	err  error
}

func (p *publishOutcome) Error() string {
	if p.err != nil {
		return p.err.Error()
	}
	return "relay: publish outcome"
}

func publishOne(ctx context.Context, db *sql.DB, pub Publisher, owner string, rec Record) error {
	dbCtx := context.WithoutCancel(ctx)

	env, err := event.Parse(rec.EventBytes)
	if err != nil {
		code := "contract_invalid"
		if errors.Is(err, event.ErrMalformed) {
			code = "malformed_envelope"
		} else if errors.Is(err, event.ErrUnsupported) {
			code = "unsupported_contract"
		}
		if qerr := Quarantine(dbCtx, db, rec.EventID, owner, code); qerr != nil {
			return qerr
		}
		return &publishOutcome{kind: outcomeQuarantine, err: err}
	}

	if env.TenantID != rec.TenantID || env.AggregateType != rec.AggregateType || env.AggregateID != rec.AggregateID {
		qerr := Quarantine(dbCtx, db, rec.EventID, owner, "aggregate_key_mismatch")
		if qerr != nil {
			return qerr
		}
		return &publishOutcome{
			kind: outcomeQuarantine,
			err:  fmt.Errorf("relay: outbox columns disagree with event bytes for %s", rec.EventID),
		}
	}

	key := []byte(AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))
	if err := pub.Publish(ctx, Topic, key, rec.EventBytes); err != nil {
		// Persist retry state even if the publish context already expired.
		if rerr := ReleaseTransient(dbCtx, db, rec.EventID, owner, "broker_publish_failed"); rerr != nil {
			return rerr
		}
		return &publishOutcome{kind: outcomeTransient, err: err}
	}

	// testSkipMarkPublished simulates the ack-before-status crash window.
	if testSkipMarkPublished != nil {
		if err := testSkipMarkPublished(); err != nil {
			return &publishOutcome{kind: outcomeAmbiguous, err: err}
		}
	}

	if err := MarkPublished(dbCtx, db, rec.EventID, owner); err != nil {
		// Broker already accepted the record. Leaving status publishing until
		// lease expiry allows duplicate-safe retry; do not erase intent.
		return &publishOutcome{kind: outcomeAmbiguous, err: err}
	}
	return &publishOutcome{kind: outcomePublished}
}

// testSkipMarkPublished, when set by same-package tests, runs after broker ACK
// and before MarkPublished. A non-nil error leaves the row publishing (ambiguous).
var testSkipMarkPublished func() error

func setTestSkipMarkPublishedForTest(fn func() error) {
	testSkipMarkPublished = fn
}
