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
	Fetched       int
	Processed     int
	Acked         int
	Skipped       int
	AckWithheld   int
	AckCommitFail bool
	// ObservedLag is the sum, across partitions that returned records in this
	// poll, of broker high watermark minus the first returned offset. It is an
	// observed fetch snapshot, not a continuous consumer-group lag reading.
	ObservedLag int64
	LagKnown    bool
}

// testSkipOffsetCommit, when set by same-package tests, runs after a successful
// durable decision and before CommitRecords. A non-nil error skips acknowledgement.
var testSkipOffsetCommit func() error

func setTestSkipOffsetCommitForTest(fn func() error) {
	testSkipOffsetCommit = fn
}

// NewConsumer dials seed brokers for consume-only use with manual commits.
func NewConsumer(db *sql.DB, seeds ...string) (*Consumer, error) {
	return NewConsumerWithGroup(db, ConsumerGroup, seeds...)
}

// NewConsumerWithGroup constructs a real Redpanda consumer with an explicit
// group. One-shot local tools use a fresh group so retained public fixture
// history remains replayable after an explicit disposable reset; normal
// runtime callers should use NewConsumer and ConsumerGroup.
func NewConsumerWithGroup(db *sql.DB, group string, seeds ...string) (*Consumer, error) {
	if db == nil {
		return nil, errors.New("platform: nil db")
	}
	if group == "" {
		return nil, errors.New("platform: consumer group required")
	}
	if len(seeds) == 0 {
		return nil, errors.New("platform: at least one broker seed required")
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup(group),
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
	fetches.EachPartition(func(partition kgo.FetchTopicPartition) {
		if len(partition.Records) == 0 || partition.HighWatermark < 0 {
			return
		}
		lag := partition.HighWatermark - partition.Records[0].Offset
		if lag < 0 {
			lag = 0
		}
		out.ObservedLag += lag
		out.LagKnown = true
	})
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
			out.AckWithheld++
			firstErr = err
			return
		}
		out.Processed++
		if !res.ShouldAck {
			out.Skipped++
			out.AckWithheld++
			return
		}
		if testSkipOffsetCommit != nil {
			if skipErr := testSkipOffsetCommit(); skipErr != nil {
				out.Skipped++
				out.AckWithheld++
				return
			}
		}
		toCommit = append(toCommit, r)
	})

	if len(toCommit) > 0 {
		if err := c.client.CommitRecords(ctx, toCommit...); err != nil {
			out.AckCommitFail = true
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
