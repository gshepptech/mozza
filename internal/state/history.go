package state

import (
	"fmt"
	"log/slog"
	"time"
)

// Rollback creates a new "rolled-back" record that reverts to the second-most-recent
// "deployed" record. It returns the newly created rollback record.
// An error is returned if fewer than two deployed records exist.
func (s *Store) Rollback() (*DeployRecord, error) {
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("Rollback: %w", err)
	}

	// Collect deployed records in order.
	var deployed []DeployRecord
	for _, r := range s.Records {
		if r.Status == "deployed" {
			deployed = append(deployed, r)
		}
	}

	if len(deployed) < 2 {
		return nil, fmt.Errorf("Rollback: no previous deployment to roll back to")
	}

	// The second-most-recent deployed record is the rollback target.
	target := deployed[len(deployed)-2]

	slog.Info("rolling back deployment",
		"target_id", target.ID,
		"target_version", target.Version,
		"app", target.AppName,
	)

	rollbackRecord := DeployRecord{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		AppName:     target.AppName,
		Target:      target.Target,
		Environment: target.Environment,
		Version:     target.Version,
		Timestamp:   time.Now(),
		Status:      "rolled-back",
	}

	s.Records = append(s.Records, rollbackRecord)

	if err := s.save(); err != nil {
		return nil, fmt.Errorf("Rollback: %w", err)
	}

	return &rollbackRecord, nil
}

// Promote creates a new "promoted" record in the target environment based on
// the latest record in the source environment. It returns the promoted record.
// An error is returned if no deployment exists in the source environment.
func (s *Store) Promote(from, to Environment) (*DeployRecord, error) {
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("Promote: %w", err)
	}

	// Find the latest record in the source environment.
	var source *DeployRecord
	for i := len(s.Records) - 1; i >= 0; i-- {
		if s.Records[i].Environment == from {
			source = &s.Records[i]
			break
		}
	}

	if source == nil {
		return nil, fmt.Errorf("Promote: no deployment found in %q environment", from)
	}

	slog.Info("promoting deployment",
		"source_id", source.ID,
		"from", from,
		"to", to,
		"version", source.Version,
	)

	promoted := DeployRecord{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		AppName:     source.AppName,
		Target:      source.Target,
		Environment: to,
		Version:     source.Version,
		Timestamp:   time.Now(),
		Status:      "promoted",
	}

	s.Records = append(s.Records, promoted)

	if err := s.save(); err != nil {
		return nil, fmt.Errorf("Promote: %w", err)
	}

	return &promoted, nil
}
