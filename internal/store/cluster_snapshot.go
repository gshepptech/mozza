package store

import (
	"fmt"
	"time"
)

// SaveClusterSnapshot inserts a new cluster cache snapshot.
func (s *Store) SaveClusterSnapshot(data string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO cluster_snapshots (data, created_at) VALUES (?, ?)`,
		data, now,
	)
	if err != nil {
		return fmt.Errorf("SaveClusterSnapshot: %w", err)
	}
	return nil
}

// LoadLatestClusterSnapshot returns the most recent snapshot data and its
// creation time. Returns ErrNotFound if no snapshots exist.
func (s *Store) LoadLatestClusterSnapshot() (string, time.Time, error) {
	var data, createdAt string
	err := s.db.QueryRow(
		`SELECT data, created_at FROM cluster_snapshots ORDER BY id DESC LIMIT 1`,
	).Scan(&data, &createdAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("LoadLatestClusterSnapshot: %w", err)
	}
	t := mustParseTime(createdAt)
	return data, t, nil
}

// CleanupOldClusterSnapshots deletes cluster cache snapshots older than maxAge.
func (s *Store) CleanupOldClusterSnapshots(maxAge time.Duration) error {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	_, err := s.db.Exec(
		`DELETE FROM cluster_snapshots WHERE created_at < ?`, cutoff,
	)
	if err != nil {
		return fmt.Errorf("CleanupOldSnapshots: %w", err)
	}
	return nil
}
