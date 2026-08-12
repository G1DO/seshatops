package relay

import "time"

// RedpandaImage is the immutable Redpanda v25.2.1 multi-platform index pin for
// Event Spine local and integration tooling (CONTRACTS.md §9).
const RedpandaImage = "docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07"

// RedpandaVersionLabel records the human-readable version paired with RedpandaImage.
const RedpandaVersionLabel = "v25.2.1"

// Topic is the single Event Spine Redpanda topic for source-owned outbox publication.
const Topic = "seshatops.m1.events"

// DefaultLeaseTTL is the publishing lease duration used when callers omit a TTL.
const DefaultLeaseTTL = 30 * time.Second
