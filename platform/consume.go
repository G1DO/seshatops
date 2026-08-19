package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/G1DO/seshatops/relay"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Consumer polls Redpanda and acknowledges offsets only after ProcessRecord
// reports a durable ShouldAck decision.
type Consumer struct {
	client *kgo.Client
	db     *sql.DB
}

// ConsumeResult summarizes one ConsumeOnce cycle.
type ConsumeResult struct {
	Fetched   int
	Processed int
	Acked     int
	Skipped   int
}

// testSkipOffsetCommit, when set by same-package tests, runs after a successful
// durable decision and before CommitRecords. A non-nil error skips acknowledgement.
var testSkipOffsetCommit func() error

func setTestSkipOffsetCommitForTest(fn func() error) {
	testSkipOffsetCommit = fn
}

// NewConsumer dials seed brokers for consume-only use with manual commits.
func NewConsumer(db *sql.DB, seeds ...string) (*Consumer, error) {
	if db == nil {
		return nil, errors.New("platform: nil db")
	}
	if len(seeds) == 0 {
		return nil, errors.New("platform: at least one broker seed required")
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup(ConsumerGroup),
		kgo.ConsumeTopics(relay.Topic),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DialTimeout(3*time.Second),
		kgo.ConnIdleTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("platform: franz consumer: %w", err)
	}
	return &Consumer{client: cl, db: db}, nil
}

// Close releases the underlying client.
func (c *Consumer) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

// Ping checks broker reachability without consuming or acknowledging records.
func (c *Consumer) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("platform: nil consumer")
	}
	return c.client.Ping(ctx)
}

// ConsumeOnce polls available records, processes each, and commits offsets only
// for durable ShouldAck decisions. Transient failures leave offsets uncommitted.
func (c *Consumer) ConsumeOnce(ctx context.Context) (ConsumeResult, error) {
	if c == nil || c.client == nil {
		return ConsumeResult{}, errors.New("platform: nil consumer")
	}
	fetches := c.client.PollFetches(ctx)
	if errs := fetches.Errors(); len(errs) > 0 {
		for _, fe := range errs {
			if errors.Is(fe.Err, context.Canceled) || errors.Is(fe.Err, context.DeadlineExceeded) {
				return ConsumeResult{}, fe.Err
			}
			return ConsumeResult{}, fmt.Errorf("platform: fetch: %w", fe.Err)
		}
	}

	var out ConsumeResult
	var toCommit []*kgo.Record
	var firstErr error
	fetches.EachRecord(func(r *kgo.Record) {
		if firstErr != nil {
			return
		}
		out.Fetched++
		res, err := ProcessRecord(ctx, c.db, r.Key, r.Value, SourcePosition{
			Topic:     r.Topic,
			Partition: r.Partition,
			Offset:    r.Offset,
		})
		if err != nil && !res.ShouldAck {
			out.Skipped++
			firstErr = err
			return
		}
		out.Processed++
		if !res.ShouldAck {
			out.Skipped++
			return
		}
		if testSkipOffsetCommit != nil {
			if skipErr := testSkipOffsetCommit(); skipErr != nil {
				out.Skipped++
				return
			}
		}
		toCommit = append(toCommit, r)
	})

	if len(toCommit) > 0 {
		if err := c.client.CommitRecords(ctx, toCommit...); err != nil {
			return out, fmt.Errorf("platform: commit offsets: %w", err)
		}
		out.Acked = len(toCommit)
	}
	return out, firstErr
}

// ProcessAndMaybeAck processes one already-fetched record and commits its
// offset only when ShouldAck is true. Used by tests that drive consumption
// explicitly.
func (c *Consumer) ProcessAndMaybeAck(ctx context.Context, r *kgo.Record) (Result, error) {
	if c == nil || c.client == nil {
		return Result{}, errors.New("platform: nil consumer")
	}
	res, err := ProcessRecord(ctx, c.db, r.Key, r.Value, SourcePosition{
		Topic:     r.Topic,
		Partition: r.Partition,
		Offset:    r.Offset,
	})
	if err != nil && !res.ShouldAck {
		return res, err
	}
	if !res.ShouldAck {
		return res, err
	}
	if testSkipOffsetCommit != nil {
		if skipErr := testSkipOffsetCommit(); skipErr != nil {
			return res, skipErr
		}
	}
	if err := c.client.CommitRecords(ctx, r); err != nil {
		return res, fmt.Errorf("platform: commit offsets: %w", err)
	}
	return res, err
}
