package store

import (
	"context"
	"fmt"
	"time"
)

// Metric represents a point-in-time resource measurement for an app.
type Metric struct {
	ID           int64   `json:"id"`
	AppID        int64   `json:"app_id"`
	Timestamp    int64   `json:"timestamp"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryBytes  int64   `json:"memory_bytes"`
	NetworkRx    int64   `json:"network_rx"`
	NetworkTx    int64   `json:"network_tx"`
	RequestCount int64   `json:"request_count"`
}

// RecordMetric inserts a new metric data point.
func (s *Store) RecordMetric(ctx context.Context, appID int64, timestamp int64, cpuPercent float64, memoryBytes, networkRx, networkTx, requestCount int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metrics (app_id, timestamp, cpu_percent, memory_bytes, network_rx, network_tx, request_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		appID, timestamp, cpuPercent, memoryBytes, networkRx, networkTx, requestCount,
	)
	if err != nil {
		return fmt.Errorf("RecordMetric: %w", err)
	}
	return nil
}

// QueryMetrics returns metrics for an app within a time range, ordered by timestamp.
func (s *Store) QueryMetrics(ctx context.Context, appID int64, start, end int64) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, timestamp, cpu_percent, memory_bytes, network_rx, network_tx, request_count
		 FROM metrics WHERE app_id = ? AND timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`,
		appID, start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("QueryMetrics: %w", err)
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.AppID, &m.Timestamp, &m.CPUPercent,
			&m.MemoryBytes, &m.NetworkRx, &m.NetworkTx, &m.RequestCount); err != nil {
			return nil, fmt.Errorf("QueryMetrics: scan: %w", err)
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// PruneMetrics deletes metrics older than maxAge.
func (s *Store) PruneMetrics(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge).Unix()

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM metrics WHERE timestamp < ?`, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("PruneMetrics: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("PruneMetrics: rows affected: %w", err)
	}
	return n, nil
}
