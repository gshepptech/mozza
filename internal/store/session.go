package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Session represents an active user session.
type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// sessionDuration is how long sessions remain valid.
const sessionDuration = 7 * 24 * time.Hour

// CreateSession creates a new session for the given user.
func (s *Store) CreateSession(userID string) (*Session, error) {
	now := time.Now().UTC()
	sess := &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		ExpiresAt: now.Add(sessionDuration),
		CreatedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.UserID,
		sess.ExpiresAt.Format(time.RFC3339), sess.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("CreateSession: %w", err)
	}
	return sess, nil
}

// SessionByID returns a session by ID. Returns ErrNotFound if expired or missing.
func (s *Store) SessionByID(id string) (*Session, error) {
	var sess Session
	var expiresAt, createdAt string
	err := s.db.QueryRow(
		`SELECT id, user_id, expires_at, created_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.UserID, &expiresAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("SessionByID: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("SessionByID: %w", err)
	}

	sess.ExpiresAt = mustParseTime(expiresAt)
	sess.CreatedAt = mustParseTime(createdAt)

	if time.Now().UTC().After(sess.ExpiresAt) {
		// Clean up expired session.
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
		return nil, fmt.Errorf("SessionByID: %w", ErrNotFound)
	}

	return &sess, nil
}

// DeleteSession removes a session by ID.
func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteSession: %w", err)
	}
	return nil
}

// DeleteUserSessions removes all sessions for a user.
func (s *Store) DeleteUserSessions(userID string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("DeleteUserSessions: %w", err)
	}
	return nil
}

// CleanExpiredSessions removes all expired sessions.
func (s *Store) CleanExpiredSessions() error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
	if err != nil {
		return fmt.Errorf("CleanExpiredSessions: %w", err)
	}
	return nil
}
