package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Deployment represents a deployment attempt.
type Deployment struct {
	ID            string
	RecipeID      string
	TeamID        string
	Target        string
	Environment   string
	ClusterID     string
	Status        string
	StartedBy     string
	Log           string
	OrderNumber   int
	Version       int
	PreviousState string
	AccessURL     string
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// CreateDeployment inserts a new deployment record. It auto-increments the
// order_number per team so deployments have a human-friendly sequence number.
// The SELECT MAX + INSERT runs inside a transaction to prevent duplicate
// order numbers under concurrent deploys.
func (s *Store) CreateDeployment(recipeID, teamID, target, env, startedBy, clusterID string) (*Deployment, error) {
	now := time.Now().UTC()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("CreateDeployment: begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("rollback failed", "err", err)
		}
	}()

	var orderNumber int
	err = tx.QueryRow(
		`SELECT COALESCE(MAX(order_number), 0) + 1 FROM deployments WHERE team_id = ?`,
		teamID,
	).Scan(&orderNumber)
	if err != nil {
		return nil, fmt.Errorf("CreateDeployment: compute order_number: %w", err)
	}

	// Auto-increment version per recipe+environment combo.
	var version int
	err = tx.QueryRow(
		`SELECT COALESCE(MAX(version), 0) + 1 FROM deployments WHERE recipe_id = ? AND environment = ?`,
		recipeID, env,
	).Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("CreateDeployment: compute version: %w", err)
	}

	d := &Deployment{
		ID:          uuid.New().String(),
		RecipeID:    recipeID,
		TeamID:      teamID,
		Target:      target,
		Environment: env,
		ClusterID:   clusterID,
		Status:      "pending",
		StartedBy:   startedBy,
		OrderNumber: orderNumber,
		Version:     version,
		StartedAt:   now,
	}

	_, err = tx.Exec(
		`INSERT INTO deployments (id, recipe_id, team_id, target, environment, cluster_id, status, started_by, log, started_at, order_number, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.RecipeID, d.TeamID, d.Target, d.Environment, d.ClusterID, d.Status, d.StartedBy,
		"", d.StartedAt.Format(time.RFC3339), d.OrderNumber, d.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateDeployment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("CreateDeployment: commit: %w", err)
	}

	return d, nil
}

// DeploymentByID returns a deployment by ID.
func (s *Store) DeploymentByID(id string) (*Deployment, error) {
	return s.scanDeployment(s.db.QueryRow(
		`SELECT id, recipe_id, team_id, target, environment, cluster_id, status, started_by, log, started_at, finished_at, order_number, version, previous_state, access_url
		 FROM deployments WHERE id = ?`, id,
	))
}

// DeploymentsForTeam returns deployments for a team, newest first.
func (s *Store) DeploymentsForTeam(teamID string, limit int) ([]Deployment, error) {
	rows, err := s.db.Query(
		`SELECT id, recipe_id, team_id, target, environment, cluster_id, status, started_by, log, started_at, finished_at, order_number, version, previous_state, access_url
		 FROM deployments WHERE team_id = ? ORDER BY started_at DESC LIMIT ?`,
		teamID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("DeploymentsForTeam: %w", err)
	}
	defer rows.Close()

	var deployments []Deployment
	for rows.Next() {
		d, err := s.scanDeploymentRow(rows)
		if err != nil {
			return nil, fmt.Errorf("DeploymentsForTeam: %w", err)
		}
		deployments = append(deployments, *d)
	}
	return deployments, rows.Err()
}

// UpdateDeploymentStatus updates the status and optionally the log and finish time.
func (s *Store) UpdateDeploymentStatus(id, status, log string, finished bool) error {
	var finishedAt *string
	if finished {
		t := time.Now().UTC().Format(time.RFC3339)
		finishedAt = &t
	}

	var (
		res sql.Result
		err error
	)
	if log != "" {
		res, err = s.db.Exec(
			`UPDATE deployments SET status = ?, log = ?, finished_at = ? WHERE id = ?`,
			status, log, finishedAt, id,
		)
	} else {
		res, err = s.db.Exec(
			`UPDATE deployments SET status = ?, finished_at = ? WHERE id = ?`,
			status, finishedAt, id,
		)
	}
	if err != nil {
		return fmt.Errorf("UpdateDeploymentStatus: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("UpdateDeploymentStatus: %w", ErrNotFound)
	}
	return nil
}

// AppendDeploymentLog appends text to a deployment's log.
func (s *Store) AppendDeploymentLog(id, text string) error {
	res, err := s.db.Exec(
		`UPDATE deployments SET log = log || ? WHERE id = ?`, text, id,
	)
	if err != nil {
		return fmt.Errorf("AppendDeploymentLog: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("AppendDeploymentLog: %w", ErrNotFound)
	}
	return nil
}

// DeleteDeployment removes a deployment by ID.
func (s *Store) DeleteDeployment(id string) error {
	res, err := s.db.Exec(`DELETE FROM deployments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteDeployment: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("DeleteDeployment: %w", ErrNotFound)
	}
	return nil
}

// PendingDeployments returns the oldest pending deployment, if any.
// It returns at most one deployment to allow the operator to process them
// one at a time in FIFO order.
func (s *Store) PendingDeployments() ([]Deployment, error) {
	rows, err := s.db.Query(
		`SELECT id, recipe_id, team_id, target, environment, cluster_id, status, started_by, log, started_at, finished_at, order_number, version, previous_state, access_url
		 FROM deployments WHERE status = 'pending' ORDER BY started_at LIMIT 1`,
	)
	if err != nil {
		return nil, fmt.Errorf("PendingDeployments: %w", err)
	}
	defer rows.Close()

	var deployments []Deployment
	for rows.Next() {
		d, err := s.scanDeploymentRow(rows)
		if err != nil {
			return nil, fmt.Errorf("PendingDeployments: %w", err)
		}
		deployments = append(deployments, *d)
	}
	return deployments, rows.Err()
}

// ClaimDeployment atomically transitions a deployment from pending to running.
// It returns ErrNotFound if the deployment does not exist or is no longer pending
// (i.e., another operator already claimed it).
func (s *Store) ClaimDeployment(id string) error {
	res, err := s.db.Exec(
		`UPDATE deployments SET status = 'running' WHERE id = ? AND status = 'pending'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("ClaimDeployment: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("ClaimDeployment: %w", ErrNotFound)
	}
	return nil
}

// maxSnapshotSize is the maximum size of a namespace snapshot in bytes (5MB).
const maxSnapshotSize = 5 * 1024 * 1024

// SetDeploymentPreviousState stores a namespace snapshot on a deployment record
// so it can be used for rollback later. Snapshots exceeding 5MB are truncated
// with a warning — rollback will be unavailable for that deployment.
func (s *Store) SetDeploymentPreviousState(id, previousState string) error {
	if len(previousState) > maxSnapshotSize {
		slog.Warn("snapshot too large for rollback, skipping",
			"deploy_id", id,
			"size_bytes", len(previousState),
			"max_bytes", maxSnapshotSize,
		)
		previousState = ""
	}
	res, err := s.db.Exec(
		`UPDATE deployments SET previous_state = ? WHERE id = ?`,
		previousState, id,
	)
	if err != nil {
		return fmt.Errorf("SetDeploymentPreviousState: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("SetDeploymentPreviousState: %w", ErrNotFound)
	}
	return nil
}

// CleanupOldSnapshots removes previous_state from deployments older than the
// most recent N per recipe+environment combo, freeing disk space.
func (s *Store) CleanupOldSnapshots(keepPerApp int) (int64, error) {
	res, err := s.db.Exec(`
		UPDATE deployments SET previous_state = ''
		WHERE previous_state != '' AND id NOT IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (
					PARTITION BY recipe_id, environment
					ORDER BY started_at DESC
				) AS rn FROM deployments WHERE previous_state != ''
			) WHERE rn <= ?
		)`, keepPerApp)
	if err != nil {
		return 0, fmt.Errorf("CleanupOldSnapshots: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return 0, fmt.Errorf("CleanupOldSnapshots: rows affected: %w", raErr)
	}
	return n, nil
}

func (s *Store) scanDeployment(row *sql.Row) (*Deployment, error) {
	var d Deployment
	var startedAt string
	var finishedAt sql.NullString
	err := row.Scan(&d.ID, &d.RecipeID, &d.TeamID, &d.Target, &d.Environment,
		&d.ClusterID, &d.Status, &d.StartedBy, &d.Log, &startedAt, &finishedAt, &d.OrderNumber, &d.Version, &d.PreviousState, &d.AccessURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanDeployment: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanDeployment: %w", err)
	}
	d.StartedAt = mustParseTime(startedAt)
	if finishedAt.Valid {
		t := mustParseTime(finishedAt.String)
		d.FinishedAt = &t
	}
	return &d, nil
}

func (s *Store) scanDeploymentRow(rows *sql.Rows) (*Deployment, error) {
	var d Deployment
	var startedAt string
	var finishedAt sql.NullString
	err := rows.Scan(&d.ID, &d.RecipeID, &d.TeamID, &d.Target, &d.Environment,
		&d.ClusterID, &d.Status, &d.StartedBy, &d.Log, &startedAt, &finishedAt, &d.OrderNumber, &d.Version, &d.PreviousState, &d.AccessURL)
	if err != nil {
		return nil, fmt.Errorf("scanDeploymentRow: %w", err)
	}
	d.StartedAt = mustParseTime(startedAt)
	if finishedAt.Valid {
		t := mustParseTime(finishedAt.String)
		d.FinishedAt = &t
	}
	return &d, nil
}

// SetDeploymentAccessURL sets the detected access URL for a deployment.
func (s *Store) SetDeploymentAccessURL(id, url string) error {
	res, err := s.db.Exec(`UPDATE deployments SET access_url = ? WHERE id = ?`, url, id)
	if err != nil {
		return fmt.Errorf("SetDeploymentAccessURL: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("SetDeploymentAccessURL: %w", ErrNotFound)
	}
	return nil
}
