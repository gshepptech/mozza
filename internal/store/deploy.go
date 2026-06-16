package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Deploy represents a deploy pipeline execution.
type Deploy struct {
	ID            string
	AppName       string
	Target        string
	K8sContext    string
	Namespace     string
	RecipeContent string
	Status        string
	TriggeredBy   string
	ErrorMessage  string
	DurationMs    int64
	CreatedAt     time.Time
	CompletedAt   *time.Time
	RollbackOf    string
}

// DeployResource represents a K8s resource created during a deploy.
type DeployResource struct {
	ID        int64
	DeployID  string
	Kind      string
	Name      string
	Namespace string
	Status    string
	AppliedAt time.Time
}

// DeployImage represents an image used in a deploy.
type DeployImage struct {
	ID        int64
	DeployID  string
	SliceName string
	Image     string
}

// CreateDeploy inserts a new deploy record and returns it.
func (s *Store) CreateDeploy(appName, target, k8sContext, namespace, recipeContent, triggeredBy string) (*Deploy, error) {
	now := time.Now().UTC()
	d := &Deploy{
		ID:            fmt.Sprintf("d-%s-%s", now.Format("20060102-150405"), uuid.New().String()[:8]),
		AppName:       appName,
		Target:        target,
		K8sContext:    k8sContext,
		Namespace:     namespace,
		RecipeContent: recipeContent,
		Status:        "in_progress",
		TriggeredBy:   triggeredBy,
		CreatedAt:     now,
	}

	_, err := s.db.Exec(
		`INSERT INTO deploys (id, app_name, target, k8s_context, namespace, recipe_content, status, triggered_by, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		d.ID, d.AppName, d.Target, d.K8sContext, d.Namespace,
		d.RecipeContent, d.Status, d.TriggeredBy, d.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("CreateDeploy: %w", err)
	}

	return d, nil
}

// CompleteDeploy marks a deploy as complete with a status and optional error.
func (s *Store) CompleteDeploy(id, status, errMsg string, durationMs int64) error {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.Exec(
		`UPDATE deploys SET status = ?, error_message = ?, duration_ms = ?, completed_at = ? WHERE id = ?`,
		status, nullableString(errMsg), durationMs, now, id,
	)
	if err != nil {
		return fmt.Errorf("CompleteDeploy: %w", err)
	}

	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("CompleteDeploy: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("CompleteDeploy: %w", ErrNotFound)
	}

	return nil
}

// SetRollbackOf updates the rollback_of field on a deploy record.
func (s *Store) SetRollbackOf(deployID, rollbackOfID string) error {
	_, err := s.db.Exec(`UPDATE deploys SET rollback_of = ? WHERE id = ?`, rollbackOfID, deployID)
	if err != nil {
		return fmt.Errorf("SetRollbackOf: %w", err)
	}
	return nil
}

// DeployByID returns a deploy by ID.
func (s *Store) DeployByID(id string) (*Deploy, error) {
	row := s.db.QueryRow(
		`SELECT id, app_name, target, k8s_context, namespace, recipe_content, status,
		        triggered_by, error_message, duration_ms, created_at, completed_at, rollback_of
		 FROM deploys WHERE id = ?`, id,
	)
	return scanDeploy(row)
}

// DeployHistory returns recent deploys for an app, newest first.
func (s *Store) DeployHistory(appName string, limit int) ([]Deploy, error) {
	rows, err := s.db.Query(
		`SELECT id, app_name, target, k8s_context, namespace, recipe_content, status,
		        triggered_by, error_message, duration_ms, created_at, completed_at, rollback_of
		 FROM deploys WHERE app_name = ? ORDER BY created_at DESC LIMIT ?`,
		appName, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("DeployHistory: %w", err)
	}
	defer rows.Close()

	var deploys []Deploy
	for rows.Next() {
		d, err := scanDeployRow(rows)
		if err != nil {
			return nil, fmt.Errorf("DeployHistory: %w", err)
		}
		deploys = append(deploys, *d)
	}

	return deploys, rows.Err()
}

// PreviousSuccessfulDeploy returns the most recent successful deploy before the current one.
func (s *Store) PreviousSuccessfulDeploy(appName string) (*Deploy, error) {
	row := s.db.QueryRow(
		`SELECT id, app_name, target, k8s_context, namespace, recipe_content, status,
		        triggered_by, error_message, duration_ms, created_at, completed_at, rollback_of
		 FROM deploys WHERE app_name = ? AND status = 'success'
		 ORDER BY created_at DESC LIMIT 1 OFFSET 1`,
		appName,
	)
	return scanDeploy(row)
}

// LatestSuccessfulDeploy returns the most recent successful deploy for an app.
func (s *Store) LatestSuccessfulDeploy(appName string) (*Deploy, error) {
	row := s.db.QueryRow(
		`SELECT id, app_name, target, k8s_context, namespace, recipe_content, status,
		        triggered_by, error_message, duration_ms, created_at, completed_at, rollback_of
		 FROM deploys WHERE app_name = ? AND status = 'success'
		 ORDER BY created_at DESC LIMIT 1`,
		appName,
	)
	return scanDeploy(row)
}

// RecordDeployResource records a resource created during a deploy.
func (s *Store) RecordDeployResource(deployID, kind, name, namespace, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO deploy_resources (deploy_id, kind, name, namespace, status, applied_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		deployID, kind, name, namespace, status, now,
	)
	if err != nil {
		return fmt.Errorf("RecordDeployResource: %w", err)
	}
	return nil
}

// RecordDeployImage records an image used in a deploy.
func (s *Store) RecordDeployImage(deployID, sliceName, image string) error {
	_, err := s.db.Exec(
		`INSERT INTO deploy_images (deploy_id, slice_name, image) VALUES (?, ?, ?)`,
		deployID, sliceName, image,
	)
	if err != nil {
		return fmt.Errorf("RecordDeployImage: %w", err)
	}
	return nil
}

// DeployResources returns all resources for a deploy.
func (s *Store) DeployResources(deployID string) ([]DeployResource, error) {
	rows, err := s.db.Query(
		`SELECT id, deploy_id, kind, name, namespace, status, applied_at
		 FROM deploy_resources WHERE deploy_id = ? ORDER BY applied_at DESC`,
		deployID,
	)
	if err != nil {
		return nil, fmt.Errorf("DeployResources: %w", err)
	}
	defer rows.Close()

	var resources []DeployResource
	for rows.Next() {
		var r DeployResource
		var appliedAt string
		if err := rows.Scan(&r.ID, &r.DeployID, &r.Kind, &r.Name, &r.Namespace, &r.Status, &appliedAt); err != nil {
			return nil, fmt.Errorf("DeployResources: %w", err)
		}
		r.AppliedAt = mustParseTime(appliedAt)
		resources = append(resources, r)
	}

	return resources, rows.Err()
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanDeploy(row *sql.Row) (*Deploy, error) {
	var d Deploy
	var k8sCtx, ns, errMsg, completedAt, rollbackOf sql.NullString
	var createdAt string

	err := row.Scan(&d.ID, &d.AppName, &d.Target, &k8sCtx, &ns, &d.RecipeContent,
		&d.Status, &d.TriggeredBy, &errMsg, &d.DurationMs, &createdAt, &completedAt, &rollbackOf)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanDeploy: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanDeploy: %w", err)
	}

	d.CreatedAt = mustParseTime(createdAt)
	if k8sCtx.Valid {
		d.K8sContext = k8sCtx.String
	}
	if ns.Valid {
		d.Namespace = ns.String
	}
	if errMsg.Valid {
		d.ErrorMessage = errMsg.String
	}
	if completedAt.Valid {
		t := mustParseTime(completedAt.String)
		d.CompletedAt = &t
	}
	if rollbackOf.Valid {
		d.RollbackOf = rollbackOf.String
	}

	return &d, nil
}

func scanDeployRow(rows *sql.Rows) (*Deploy, error) {
	var d Deploy
	var k8sCtx, ns, errMsg, completedAt, rollbackOf sql.NullString
	var createdAt string

	err := rows.Scan(&d.ID, &d.AppName, &d.Target, &k8sCtx, &ns, &d.RecipeContent,
		&d.Status, &d.TriggeredBy, &errMsg, &d.DurationMs, &createdAt, &completedAt, &rollbackOf)
	if err != nil {
		return nil, fmt.Errorf("scanDeployRow: %w", err)
	}

	d.CreatedAt = mustParseTime(createdAt)
	if k8sCtx.Valid {
		d.K8sContext = k8sCtx.String
	}
	if ns.Valid {
		d.Namespace = ns.String
	}
	if errMsg.Valid {
		d.ErrorMessage = errMsg.String
	}
	if completedAt.Valid {
		t := mustParseTime(completedAt.String)
		d.CompletedAt = &t
	}
	if rollbackOf.Valid {
		d.RollbackOf = rollbackOf.String
	}

	return &d, nil
}
