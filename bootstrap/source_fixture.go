package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/G1DO/seshatops/northstar"
)

// SourceFixtureSummary is the bounded source-only result used by local
// demonstration tooling. It intentionally contains no event payload bytes.
type SourceFixtureSummary struct {
	FixtureVersion string
	TenantID       string
	EventIDs       []string
	SourceCount    int
	OutboxState    map[string]int
}

// AcceptSourceFixture accepts the deterministic lineage fixture through the
// real ERP source methods and stops at the transactional outbox boundary. It
// does not run a relay or consumer; a long-lived runtime owns those steps.
// Existing matching source state is reused safely.
func AcceptSourceFixture(ctx context.Context, db *sql.DB, seed string) (SourceFixtureSummary, error) {
	if db == nil {
		return SourceFixtureSummary{}, fmt.Errorf("bootstrap: database required")
	}
	fx, err := northstar.GenerateLineage(seed)
	if err != nil {
		return SourceFixtureSummary{}, err
	}
	if err := ensureInventory(ctx, db, fx); err != nil {
		return SourceFixtureSummary{}, err
	}
	if err := ensureSources(ctx, db, fx); err != nil {
		return SourceFixtureSummary{}, err
	}

	summary := SourceFixtureSummary{
		FixtureVersion: fx.Seed,
		TenantID:       fx.TenantID,
		EventIDs:       make([]string, 0, len(fx.Events)),
		OutboxState:    make(map[string]int),
	}
	for _, env := range fx.Events {
		row, found, err := loadExpectedOutbox(ctx, db, env)
		if err != nil {
			return SourceFixtureSummary{}, err
		}
		if !found {
			return SourceFixtureSummary{}, fmt.Errorf("%w: accepted source has no outbox event %s", ErrImmutableFixtureConflict, env.EventID)
		}
		if err := verifyOutbox(env, row); err != nil {
			return SourceFixtureSummary{}, err
		}
		summary.EventIDs = append(summary.EventIDs, env.EventID)
		summary.SourceCount++
		summary.OutboxState[row.status]++
	}
	return summary, nil
}

// InspectFixtureCheckpoint reads the deterministic fixture checkpoint without
// accepting source state, draining the outbox, or consuming broker records.
func InspectFixtureCheckpoint(ctx context.Context, db *sql.DB, seed string) (Summary, bool, error) {
	if db == nil {
		return Summary{}, false, fmt.Errorf("bootstrap: database required")
	}
	fx, err := northstar.GenerateLineage(seed)
	if err != nil {
		return Summary{}, false, err
	}
	return inspectCheckpoint(ctx, db, fx)
}
