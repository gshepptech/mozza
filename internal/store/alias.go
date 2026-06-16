package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ImageAlias maps a friendly name to a Docker image reference within a team.
type ImageAlias struct {
	ID        string
	TeamID    string
	AliasName string
	ImageRef  string
	CreatedAt time.Time
}

// CreateAlias inserts a new image alias for a team.
func (s *Store) CreateAlias(teamID, name, imageRef string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO image_aliases (id, team_id, alias_name, image_ref, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(), teamID, name, imageRef, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("CreateAlias: %w", ErrConflict)
		}
		return fmt.Errorf("CreateAlias: %w", err)
	}
	return nil
}

// ListAliases returns all image aliases for a team.
func (s *Store) ListAliases(teamID string) ([]ImageAlias, error) {
	rows, err := s.db.Query(
		`SELECT id, team_id, alias_name, image_ref, created_at
		 FROM image_aliases WHERE team_id = ? ORDER BY alias_name`, teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListAliases: %w", err)
	}
	defer rows.Close()

	var aliases []ImageAlias
	for rows.Next() {
		var a ImageAlias
		var createdAt string
		if err := rows.Scan(&a.ID, &a.TeamID, &a.AliasName, &a.ImageRef, &createdAt); err != nil {
			return nil, fmt.Errorf("ListAliases: %w", err)
		}
		a.CreatedAt = mustParseTime(createdAt)
		aliases = append(aliases, a)
	}
	return aliases, rows.Err()
}

// DeleteAlias removes an image alias by team and name.
func (s *Store) DeleteAlias(teamID, name string) error {
	res, err := s.db.Exec(
		`DELETE FROM image_aliases WHERE team_id = ? AND alias_name = ?`,
		teamID, name,
	)
	if err != nil {
		return fmt.Errorf("DeleteAlias: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("DeleteAlias: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("DeleteAlias: %w", ErrNotFound)
	}
	return nil
}

// AliasesForTeam returns a name->imageRef map of all aliases for a team.
func (s *Store) AliasesForTeam(teamID string) (map[string]string, error) {
	aliases, err := s.ListAliases(teamID)
	if err != nil {
		return nil, fmt.Errorf("AliasesForTeam: %w", err)
	}

	result := make(map[string]string, len(aliases))
	for _, a := range aliases {
		result[a.AliasName] = a.ImageRef
	}
	return result, nil
}
