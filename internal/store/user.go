package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User represents a registered user.
type User struct {
	ID        string
	Email     string
	Name      string
	Password  string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a unique constraint is violated.
var ErrConflict = errors.New("conflict")

// CreateUser inserts a new user record.
func (s *Store) CreateUser(email, name, password, role string) (*User, error) {
	now := time.Now().UTC()
	u := &User{
		ID:        uuid.New().String(),
		Email:     email,
		Name:      name,
		Password:  password,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO users (id, email, name, password, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.Password, u.Role,
		u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("CreateUser: %w", ErrConflict)
		}
		return nil, fmt.Errorf("CreateUser: %w", err)
	}

	return u, nil
}

// EnsureUser inserts a user with a specific ID if it doesn't already exist.
// Used by --no-auth mode to create the synthetic admin user.
func (s *Store) EnsureUser(id, email, name, role string) (*User, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO users (id, email, name, password, role, created_at, updated_at)
		 VALUES (?, ?, ?, '', ?, ?, ?)`,
		id, email, name, role, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("EnsureUser: %w", err)
	}
	return &User{ID: id, Email: email, Name: name, Role: role, CreatedAt: now, UpdatedAt: now}, nil
}

// UserByID returns a user by ID.
func (s *Store) UserByID(id string) (*User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, name, password, role, created_at, updated_at FROM users WHERE id = ?`, id,
	))
}

// UserByEmail returns a user by email.
func (s *Store) UserByEmail(email string) (*User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, name, password, role, created_at, updated_at FROM users WHERE email = ?`, email,
	))
}

// UpdateUser updates a user's name and role.
func (s *Store) UpdateUser(id, name, role string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE users SET name = ?, role = ?, updated_at = ? WHERE id = ?`,
		name, role, now, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateUser: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("UpdateUser: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("UpdateUser: %w", ErrNotFound)
	}
	return nil
}

// DeleteUser removes a user by ID.
func (s *Store) DeleteUser(id string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteUser: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("DeleteUser: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("DeleteUser: %w", ErrNotFound)
	}
	return nil
}

// ListUsers returns all users.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, email, name, password, role, created_at, updated_at FROM users ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListUsers: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := s.scanUserRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListUsers: %w", err)
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (s *Store) scanUser(row *sql.Row) (*User, error) {
	var u User
	var createdAt, updatedAt string
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.Role, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanUser: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanUser: %w", err)
	}
	u.CreatedAt = mustParseTime(createdAt)
	u.UpdatedAt = mustParseTime(updatedAt)
	return &u, nil
}

func (s *Store) scanUserRow(rows *sql.Rows) (*User, error) {
	var u User
	var createdAt, updatedAt string
	err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.Role, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanUserRow: %w", err)
	}
	u.CreatedAt = mustParseTime(createdAt)
	u.UpdatedAt = mustParseTime(updatedAt)
	return &u, nil
}

// isUniqueViolation checks if the error is a unique constraint violation
// for both SQLite and PostgreSQL.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite: "UNIQUE constraint failed: ..."
	// PostgreSQL: "... duplicate key value violates unique constraint ..."
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}
