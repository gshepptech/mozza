// Package state provides deploy state tracking and environment model.
// It persists deployment records to a local JSON file, enabling history,
// rollback, and promotion workflows without external dependencies.
package state

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// Environment represents the target deployment environment.
type Environment string

const (
	// EnvDev is the development environment.
	EnvDev Environment = "dev"
	// EnvStaging is the staging environment.
	EnvStaging Environment = "staging"
	// EnvProduction is the production environment.
	EnvProduction Environment = "production"
)

// stateFileName is the default file name for persisted deploy state.
const stateFileName = ".mozza-state.json"

// DeployRecord captures a single deployment event.
type DeployRecord struct {
	// ID is a unique identifier based on the Unix nanosecond timestamp.
	ID string `json:"id"`
	// AppName is the application that was deployed.
	AppName string `json:"app_name"`
	// Target is the deployment target (e.g. "local", "kubernetes").
	Target string `json:"target"`
	// Environment is the target environment for this deployment.
	Environment Environment `json:"environment"`
	// Version is the image tag or recipe hash for the deployment.
	Version string `json:"version"`
	// Timestamp records when the deployment occurred.
	Timestamp time.Time `json:"timestamp"`
	// Status describes the deployment outcome (e.g. "deployed", "rolled-back", "promoted").
	Status string `json:"status"`
}

// Store manages deploy state backed by a JSON file.
type Store struct {
	filePath string
	Records  []DeployRecord `json:"records"`
}

// NewStore creates a Store that persists state to dir/.mozza-state.json.
func NewStore(dir string) *Store {
	return &Store{
		filePath: dir + "/" + stateFileName,
	}
}

// Record appends a deploy record and persists the updated state to disk.
func (s *Store) Record(r DeployRecord) error {
	if err := s.load(); err != nil {
		return fmt.Errorf("Record: %w", err)
	}

	if r.ID == "" {
		r.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}

	s.Records = append(s.Records, r)

	slog.Info("recording deployment",
		"id", r.ID,
		"app", r.AppName,
		"env", r.Environment,
		"status", r.Status,
	)

	if err := s.save(); err != nil {
		return fmt.Errorf("Record: %w", err)
	}

	return nil
}

// Latest returns the most recent deploy record.
// It returns an error if no records exist.
func (s *Store) Latest() (*DeployRecord, error) {
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("Latest: %w", err)
	}

	if len(s.Records) == 0 {
		return nil, fmt.Errorf("Latest: no deploy records found")
	}

	r := s.Records[len(s.Records)-1]
	return &r, nil
}

// History returns the most recent N deploy records in reverse chronological order.
// If limit is zero or exceeds the total count, all records are returned.
func (s *Store) History(limit int) ([]DeployRecord, error) {
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("History: %w", err)
	}

	total := len(s.Records)
	if limit <= 0 || limit > total {
		limit = total
	}

	result := make([]DeployRecord, limit)
	for i := range limit {
		result[i] = s.Records[total-1-i]
	}

	return result, nil
}

// load reads deploy state from the JSON file.
// If the file does not exist, it initializes an empty record set.
func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.Records = nil
			return nil
		}
		return fmt.Errorf("load: %w", err)
	}

	if err := json.Unmarshal(data, &s.Records); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	return nil
}

// save writes the current deploy state to the JSON file.
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.Records, "", "  ")
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0o644); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	return nil
}

// ValidateEnvironment validates a raw string as a known Environment value.
// It returns the typed Environment on success or an error for unknown values.
func ValidateEnvironment(env string) (Environment, error) {
	switch Environment(env) {
	case EnvDev, EnvStaging, EnvProduction:
		return Environment(env), nil
	default:
		return "", fmt.Errorf("ValidateEnvironment: unknown environment %q", env)
	}
}
