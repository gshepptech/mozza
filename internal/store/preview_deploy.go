package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PreviewDeploy represents a preview environment for a branch.
type PreviewDeploy struct {
	ID           int64      `json:"id"`
	AppID        int64      `json:"app_id"`
	Branch       string     `json:"branch"`
	Domain       string     `json:"domain,omitempty"`
	DeployID     int64      `json:"deploy_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActivity *time.Time `json:"last_activity,omitempty"`
}

// CreatePreviewDeploy inserts a new preview deploy record.
func (s *Store) CreatePreviewDeploy(ctx context.Context, appID int64, branch, domain string, deployID int64) (*PreviewDeploy, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO preview_deploys (app_id, branch, domain, deploy_id, created_at, last_activity)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		appID, branch, nullableString(domain), deployID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("CreatePreviewDeploy: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("CreatePreviewDeploy: last insert id: %w", err)
	}

	ts := mustParseTime(now)
	return &PreviewDeploy{
		ID:           id,
		AppID:        appID,
		Branch:       branch,
		Domain:       domain,
		DeployID:     deployID,
		CreatedAt:    ts,
		LastActivity: &ts,
	}, nil
}

// GetPreviewDeploy returns a preview deploy by ID.
func (s *Store) GetPreviewDeploy(ctx context.Context, id int64) (*PreviewDeploy, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, app_id, branch, domain, deploy_id, created_at, last_activity
		 FROM preview_deploys WHERE id = ?`, id,
	)
	return scanPreviewDeploy(row)
}

// ListPreviewDeploys returns all preview deploys, newest first.
func (s *Store) ListPreviewDeploys(ctx context.Context) ([]PreviewDeploy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, branch, domain, deploy_id, created_at, last_activity
		 FROM preview_deploys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListPreviewDeploys: %w", err)
	}
	defer rows.Close()

	var deploys []PreviewDeploy
	for rows.Next() {
		d, err := scanPreviewDeployRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListPreviewDeploys: %w", err)
		}
		deploys = append(deploys, *d)
	}
	return deploys, rows.Err()
}

// ListPreviewDeploysByApp returns preview deploys for a specific app.
func (s *Store) ListPreviewDeploysByApp(ctx context.Context, appID int64) ([]PreviewDeploy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, branch, domain, deploy_id, created_at, last_activity
		 FROM preview_deploys WHERE app_id = ? ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListPreviewDeploysByApp: %w", err)
	}
	defer rows.Close()

	var deploys []PreviewDeploy
	for rows.Next() {
		d, err := scanPreviewDeployRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListPreviewDeploysByApp: %w", err)
		}
		deploys = append(deploys, *d)
	}
	return deploys, rows.Err()
}

// DeletePreviewDeploy removes a preview deploy by ID.
func (s *Store) DeletePreviewDeploy(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM preview_deploys WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("DeletePreviewDeploy: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeletePreviewDeploy: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("DeletePreviewDeploy: %w", ErrNotFound)
	}
	return nil
}

// CleanStalePreviewDeploys removes preview deploys with no activity since the cutoff.
func (s *Store) CleanStalePreviewDeploys(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM preview_deploys WHERE last_activity < ?`,
		cutoff.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("CleanStalePreviewDeploys: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("CleanStalePreviewDeploys: rows affected: %w", err)
	}
	return n, nil
}

// FindPreviewDeployByAppAndBranch returns a preview deploy matching the app and branch.
// Returns ErrNotFound when no match exists.
func (s *Store) FindPreviewDeployByAppAndBranch(ctx context.Context, appID int64, branch string) (*PreviewDeploy, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, app_id, branch, domain, deploy_id, created_at, last_activity
		 FROM preview_deploys WHERE app_id = ? AND branch = ?`, appID, branch,
	)
	return scanPreviewDeploy(row)
}

// CountPreviewDeploysByApp returns the number of preview deploys for an app.
func (s *Store) CountPreviewDeploysByApp(ctx context.Context, appID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM preview_deploys WHERE app_id = ?`, appID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountPreviewDeploysByApp: %w", err)
	}
	return count, nil
}

// UpdatePreviewDeployActivity updates the last_activity timestamp for a preview deploy.
func (s *Store) UpdatePreviewDeployActivity(ctx context.Context, id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE preview_deploys SET last_activity = ? WHERE id = ?`, now, id,
	)
	if err != nil {
		return fmt.Errorf("UpdatePreviewDeployActivity: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdatePreviewDeployActivity: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("UpdatePreviewDeployActivity: %w", ErrNotFound)
	}
	return nil
}

// DeletePreviewDeployByAppAndBranch removes a preview deploy matching the app and branch.
func (s *Store) DeletePreviewDeployByAppAndBranch(ctx context.Context, appID int64, branch string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM preview_deploys WHERE app_id = ? AND branch = ?`, appID, branch,
	)
	if err != nil {
		return fmt.Errorf("DeletePreviewDeployByAppAndBranch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeletePreviewDeployByAppAndBranch: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("DeletePreviewDeployByAppAndBranch: %w", ErrNotFound)
	}
	return nil
}

func scanPreviewDeploy(row *sql.Row) (*PreviewDeploy, error) {
	var d PreviewDeploy
	var domain, lastActivity sql.NullString
	var deployID sql.NullInt64
	var createdAt string

	err := row.Scan(&d.ID, &d.AppID, &d.Branch, &domain, &deployID, &createdAt, &lastActivity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanPreviewDeploy: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanPreviewDeploy: %w", err)
	}

	d.CreatedAt = mustParseTime(createdAt)
	if domain.Valid {
		d.Domain = domain.String
	}
	if deployID.Valid {
		d.DeployID = deployID.Int64
	}
	if lastActivity.Valid {
		t := mustParseTime(lastActivity.String)
		d.LastActivity = &t
	}
	return &d, nil
}

func scanPreviewDeployRow(rows *sql.Rows) (*PreviewDeploy, error) {
	var d PreviewDeploy
	var domain, lastActivity sql.NullString
	var deployID sql.NullInt64
	var createdAt string

	err := rows.Scan(&d.ID, &d.AppID, &d.Branch, &domain, &deployID, &createdAt, &lastActivity)
	if err != nil {
		return nil, fmt.Errorf("scanPreviewDeployRow: %w", err)
	}

	d.CreatedAt = mustParseTime(createdAt)
	if domain.Valid {
		d.Domain = domain.String
	}
	if deployID.Valid {
		d.DeployID = deployID.Int64
	}
	if lastActivity.Valid {
		t := mustParseTime(lastActivity.String)
		d.LastActivity = &t
	}
	return &d, nil
}
