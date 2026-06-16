package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ConnectedRepo represents a Git repository connected for CI/CD.
type ConnectedRepo struct {
	ID                  int64     `json:"id"`
	RepoURL             string    `json:"repo_url"`
	WebhookID           string    `json:"webhook_id,omitempty"`
	OAuthTokenEncrypted []byte    `json:"-"`
	AppID               int64     `json:"app_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CreateConnectedRepo inserts a new connected repo record.
func (s *Store) CreateConnectedRepo(ctx context.Context, repoURL, webhookID string, oauthToken []byte, appID int64) (*ConnectedRepo, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO connected_repos (repo_url, webhook_id, oauth_token_encrypted, app_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		repoURL, nullableString(webhookID), oauthToken, appID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateConnectedRepo: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("CreateConnectedRepo: last insert id: %w", err)
	}

	return &ConnectedRepo{
		ID:                  id,
		RepoURL:             repoURL,
		WebhookID:           webhookID,
		OAuthTokenEncrypted: oauthToken,
		AppID:               appID,
		CreatedAt:           mustParseTime(now),
		UpdatedAt:           mustParseTime(now),
	}, nil
}

// GetConnectedRepo returns a connected repo by ID.
func (s *Store) GetConnectedRepo(ctx context.Context, id int64) (*ConnectedRepo, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, repo_url, webhook_id, oauth_token_encrypted, app_id, created_at, updated_at
		 FROM connected_repos WHERE id = ?`, id,
	)
	return scanConnectedRepo(row)
}

// ListConnectedRepos returns all connected repos.
func (s *Store) ListConnectedRepos(ctx context.Context) ([]ConnectedRepo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repo_url, webhook_id, oauth_token_encrypted, app_id, created_at, updated_at
		 FROM connected_repos ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListConnectedRepos: %w", err)
	}
	defer rows.Close()

	var repos []ConnectedRepo
	for rows.Next() {
		r, err := scanConnectedRepoRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListConnectedRepos: %w", err)
		}
		repos = append(repos, *r)
	}
	return repos, rows.Err()
}

// DeleteConnectedRepo removes a connected repo by ID.
func (s *Store) DeleteConnectedRepo(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM connected_repos WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("DeleteConnectedRepo: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteConnectedRepo: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("DeleteConnectedRepo: %w", ErrNotFound)
	}
	return nil
}

func scanConnectedRepo(row *sql.Row) (*ConnectedRepo, error) {
	var r ConnectedRepo
	var webhookID sql.NullString
	var token []byte
	var appID sql.NullInt64
	var createdAt, updatedAt string

	err := row.Scan(&r.ID, &r.RepoURL, &webhookID, &token, &appID, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanConnectedRepo: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanConnectedRepo: %w", err)
	}

	r.OAuthTokenEncrypted = token
	r.CreatedAt = mustParseTime(createdAt)
	r.UpdatedAt = mustParseTime(updatedAt)
	if webhookID.Valid {
		r.WebhookID = webhookID.String
	}
	if appID.Valid {
		r.AppID = appID.Int64
	}
	return &r, nil
}

func scanConnectedRepoRow(rows *sql.Rows) (*ConnectedRepo, error) {
	var r ConnectedRepo
	var webhookID sql.NullString
	var token []byte
	var appID sql.NullInt64
	var createdAt, updatedAt string

	err := rows.Scan(&r.ID, &r.RepoURL, &webhookID, &token, &appID, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanConnectedRepoRow: %w", err)
	}

	r.OAuthTokenEncrypted = token
	r.CreatedAt = mustParseTime(createdAt)
	r.UpdatedAt = mustParseTime(updatedAt)
	if webhookID.Valid {
		r.WebhookID = webhookID.String
	}
	if appID.Valid {
		r.AppID = appID.Int64
	}
	return &r, nil
}
