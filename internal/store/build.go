package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Build represents a container image build.
type Build struct {
	ID         int64     `json:"id"`
	RepoURL    string    `json:"repo_url"`
	CommitSHA  string    `json:"commit_sha,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	Status     string    `json:"status"`
	Logs       string    `json:"logs,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	ImageTag   string    `json:"image_tag,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateBuild inserts a new build record.
func (s *Store) CreateBuild(ctx context.Context, repoURL, commitSHA, branch string) (*Build, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO builds (repo_url, commit_sha, branch, status, created_at)
		 VALUES (?, ?, ?, 'pending', ?)`,
		repoURL, commitSHA, branch, now,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateBuild: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("CreateBuild: last insert id: %w", err)
	}

	return &Build{
		ID:        id,
		RepoURL:   repoURL,
		CommitSHA: commitSHA,
		Branch:    branch,
		Status:    "pending",
		CreatedAt: mustParseTime(now),
	}, nil
}

// GetBuild returns a build by ID.
func (s *Store) GetBuild(ctx context.Context, id int64) (*Build, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, repo_url, commit_sha, branch, status, logs, duration_ms, image_tag, created_at
		 FROM builds WHERE id = ?`, id,
	)
	return scanBuild(row)
}

// ListBuilds returns recent builds, newest first.
func (s *Store) ListBuilds(ctx context.Context, limit int) ([]Build, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repo_url, commit_sha, branch, status, logs, duration_ms, image_tag, created_at
		 FROM builds ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ListBuilds: %w", err)
	}
	defer rows.Close()

	var builds []Build
	for rows.Next() {
		b, err := scanBuildRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListBuilds: %w", err)
		}
		builds = append(builds, *b)
	}
	return builds, rows.Err()
}

// UpdateBuild updates a build's status, logs, duration, and image tag.
func (s *Store) UpdateBuild(ctx context.Context, id int64, status, logs string, durationMs int64, imageTag string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE builds SET status = ?, logs = ?, duration_ms = ?, image_tag = ? WHERE id = ?`,
		status, logs, durationMs, imageTag, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateBuild: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateBuild: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("UpdateBuild: %w", ErrNotFound)
	}
	return nil
}

// ListBuildsByRepo returns builds for a specific repo, newest first.
func (s *Store) ListBuildsByRepo(ctx context.Context, repoURL string, limit int) ([]Build, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repo_url, commit_sha, branch, status, logs, duration_ms, image_tag, created_at
		 FROM builds WHERE repo_url = ? ORDER BY created_at DESC LIMIT ?`,
		repoURL, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ListBuildsByRepo: %w", err)
	}
	defer rows.Close()

	var builds []Build
	for rows.Next() {
		b, err := scanBuildRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListBuildsByRepo: %w", err)
		}
		builds = append(builds, *b)
	}
	return builds, rows.Err()
}

// GetLatestBuildByBranch returns the most recent build for a repo+branch.
func (s *Store) GetLatestBuildByBranch(ctx context.Context, repoURL, branch string) (*Build, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, repo_url, commit_sha, branch, status, logs, duration_ms, image_tag, created_at
		 FROM builds WHERE repo_url = ? AND branch = ?
		 ORDER BY created_at DESC LIMIT 1`,
		repoURL, branch,
	)
	return scanBuild(row)
}

func scanBuild(row *sql.Row) (*Build, error) {
	var b Build
	var commitSHA, branch, logs, imageTag sql.NullString
	var durationMs sql.NullInt64
	var createdAt string

	err := row.Scan(&b.ID, &b.RepoURL, &commitSHA, &branch, &b.Status,
		&logs, &durationMs, &imageTag, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanBuild: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanBuild: %w", err)
	}

	b.CreatedAt = mustParseTime(createdAt)
	if commitSHA.Valid {
		b.CommitSHA = commitSHA.String
	}
	if branch.Valid {
		b.Branch = branch.String
	}
	if logs.Valid {
		b.Logs = logs.String
	}
	if durationMs.Valid {
		b.DurationMs = durationMs.Int64
	}
	if imageTag.Valid {
		b.ImageTag = imageTag.String
	}
	return &b, nil
}

func scanBuildRow(rows *sql.Rows) (*Build, error) {
	var b Build
	var commitSHA, branch, logs, imageTag sql.NullString
	var durationMs sql.NullInt64
	var createdAt string

	err := rows.Scan(&b.ID, &b.RepoURL, &commitSHA, &branch, &b.Status,
		&logs, &durationMs, &imageTag, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scanBuildRow: %w", err)
	}

	b.CreatedAt = mustParseTime(createdAt)
	if commitSHA.Valid {
		b.CommitSHA = commitSHA.String
	}
	if branch.Valid {
		b.Branch = branch.String
	}
	if logs.Valid {
		b.Logs = logs.String
	}
	if durationMs.Valid {
		b.DurationMs = durationMs.Int64
	}
	if imageTag.Valid {
		b.ImageTag = imageTag.String
	}
	return &b, nil
}
