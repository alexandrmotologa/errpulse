package storage

import (
	"context"
	"fmt"
	"log"
	"time"
)

// PruneOldEvents removes event records older than the specified retention duration in days.
func (db *DB) PruneOldEvents(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	res, err := db.Exec(`DELETE FROM events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed pruning old events: %w", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	if deleted > 0 {
		// Run WAL checkpoint to reclaim pages
		_, _ = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}

	return deleted, nil
}

// StartPruningJob periodically invokes PruneOldEvents in the background.
func (db *DB) StartPruningJob(ctx context.Context, interval time.Duration, retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count, err := db.PruneOldEvents(retentionDays)
				if err != nil {
					log.Printf("[ErrPulse] Retention pruner error: %v", err)
				} else if count > 0 {
					log.Printf("[ErrPulse] Pruned %d events older than %d days", count, retentionDays)
				}
			}
		}
	}()
}
