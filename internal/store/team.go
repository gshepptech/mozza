package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Team represents a team that owns recipes and deployments.
type Team struct {
	ID        string
	Name      string
	Slug      string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TeamMember represents a user's membership in a team.
type TeamMember struct {
	TeamID   string
	UserID   string
	Role     string
	JoinedAt time.Time
}

// CreateTeam creates a new team and adds the creator as owner.
func (s *Store) CreateTeam(name, slug, createdBy string) (*Team, error) {
	now := time.Now().UTC()
	t := &Team{
		ID:        uuid.New().String(),
		Name:      name,
		Slug:      slug,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("CreateTeam: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("rollback failed", "err", err)
		}
	}()

	_, err = tx.Exec(
		`INSERT INTO teams (id, name, slug, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Slug, t.CreatedBy,
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("CreateTeam: %w", ErrConflict)
		}
		return nil, fmt.Errorf("CreateTeam: %w", err)
	}

	// Add creator as owner.
	_, err = tx.Exec(
		`INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		t.ID, createdBy, "owner", now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("CreateTeam: add owner: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("CreateTeam: commit: %w", err)
	}

	return t, nil
}

// TeamByID returns a team by ID.
func (s *Store) TeamByID(id string) (*Team, error) {
	return s.scanTeam(s.db.QueryRow(
		`SELECT id, name, slug, created_by, created_at, updated_at FROM teams WHERE id = ?`, id,
	))
}

// TeamBySlug returns a team by its URL slug.
func (s *Store) TeamBySlug(slug string) (*Team, error) {
	return s.scanTeam(s.db.QueryRow(
		`SELECT id, name, slug, created_by, created_at, updated_at FROM teams WHERE slug = ?`, slug,
	))
}

// TeamsForUser returns all teams a user belongs to.
func (s *Store) TeamsForUser(userID string) ([]Team, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.name, t.slug, t.created_by, t.created_at, t.updated_at
		 FROM teams t JOIN team_members tm ON t.id = tm.team_id
		 WHERE tm.user_id = ? ORDER BY t.name`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("TeamsForUser: %w", err)
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		t, err := s.scanTeamRow(rows)
		if err != nil {
			return nil, fmt.Errorf("TeamsForUser: %w", err)
		}
		teams = append(teams, *t)
	}
	return teams, rows.Err()
}

// AddTeamMember adds a user to a team.
func (s *Store) AddTeamMember(teamID, userID, role string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		teamID, userID, role, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("AddTeamMember: %w", ErrConflict)
		}
		return fmt.Errorf("AddTeamMember: %w", err)
	}
	return nil
}

// RemoveTeamMember removes a user from a team.
func (s *Store) RemoveTeamMember(teamID, userID string) error {
	res, err := s.db.Exec(
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID,
	)
	if err != nil {
		return fmt.Errorf("RemoveTeamMember: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("RemoveTeamMember: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("RemoveTeamMember: %w", ErrNotFound)
	}
	return nil
}

// TeamMembers returns all members of a team.
func (s *Store) TeamMembers(teamID string) ([]TeamMember, error) {
	rows, err := s.db.Query(
		`SELECT team_id, user_id, role, joined_at FROM team_members WHERE team_id = ? ORDER BY joined_at`,
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("TeamMembers: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var m TeamMember
		var joinedAt string
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &joinedAt); err != nil {
			return nil, fmt.Errorf("TeamMembers: %w", err)
		}
		m.JoinedAt = mustParseTime(joinedAt)
		members = append(members, m)
	}
	return members, rows.Err()
}

// IsTeamMember checks if a user is a member of a team.
func (s *Store) IsTeamMember(teamID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("IsTeamMember: %w", err)
	}
	return count > 0, nil
}

// DeleteTeam removes a team and all associated data (cascades).
func (s *Store) DeleteTeam(id string) error {
	res, err := s.db.Exec(`DELETE FROM teams WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteTeam: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("DeleteTeam: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("DeleteTeam: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) scanTeam(row *sql.Row) (*Team, error) {
	var t Team
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedBy, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanTeam: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanTeam: %w", err)
	}
	t.CreatedAt = mustParseTime(createdAt)
	t.UpdatedAt = mustParseTime(updatedAt)
	return &t, nil
}

func (s *Store) scanTeamRow(rows *sql.Rows) (*Team, error) {
	var t Team
	var createdAt, updatedAt string
	err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanTeamRow: %w", err)
	}
	t.CreatedAt = mustParseTime(createdAt)
	t.UpdatedAt = mustParseTime(updatedAt)
	return &t, nil
}
