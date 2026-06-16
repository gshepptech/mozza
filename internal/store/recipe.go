package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Recipe represents a saved recipe with both source and canvas layout.
type Recipe struct {
	ID        string
	TeamID    string
	Name      string
	Source    string
	Canvas    string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateRecipe inserts a new recipe.
func (s *Store) CreateRecipe(teamID, name, source, canvas, createdBy string) (*Recipe, error) {
	now := time.Now().UTC()
	r := &Recipe{
		ID:        uuid.New().String(),
		TeamID:    teamID,
		Name:      name,
		Source:    source,
		Canvas:    canvas,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO recipes (id, team_id, name, source, canvas, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TeamID, r.Name, r.Source, r.Canvas, r.CreatedBy,
		r.CreatedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("CreateRecipe: %w", ErrConflict)
		}
		return nil, fmt.Errorf("CreateRecipe: %w", err)
	}

	return r, nil
}

// RecipeByID returns a recipe by ID.
func (s *Store) RecipeByID(id string) (*Recipe, error) {
	return s.scanRecipe(s.db.QueryRow(
		`SELECT id, team_id, name, source, canvas, created_by, created_at, updated_at
		 FROM recipes WHERE id = ?`, id,
	))
}

// RecipesForTeam returns all recipes for a team.
func (s *Store) RecipesForTeam(teamID string) ([]Recipe, error) {
	rows, err := s.db.Query(
		`SELECT id, team_id, name, source, canvas, created_by, created_at, updated_at
		 FROM recipes WHERE team_id = ? ORDER BY name`, teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("RecipesForTeam: %w", err)
	}
	defer rows.Close()

	var recipes []Recipe
	for rows.Next() {
		r, err := s.scanRecipeRow(rows)
		if err != nil {
			return nil, fmt.Errorf("RecipesForTeam: %w", err)
		}
		recipes = append(recipes, *r)
	}
	return recipes, rows.Err()
}

// UpdateRecipe updates a recipe's source, canvas, and name.
func (s *Store) UpdateRecipe(id, name, source, canvas string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE recipes SET name = ?, source = ?, canvas = ?, updated_at = ? WHERE id = ?`,
		name, source, canvas, now, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateRecipe: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("UpdateRecipe: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("UpdateRecipe: %w", ErrNotFound)
	}
	return nil
}

// DeleteRecipe removes a recipe by ID.
func (s *Store) DeleteRecipe(id string) error {
	res, err := s.db.Exec(`DELETE FROM recipes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteRecipe: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("DeleteRecipe: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("DeleteRecipe: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) scanRecipe(row *sql.Row) (*Recipe, error) {
	var r Recipe
	var createdAt, updatedAt string
	err := row.Scan(&r.ID, &r.TeamID, &r.Name, &r.Source, &r.Canvas, &r.CreatedBy, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanRecipe: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanRecipe: %w", err)
	}
	r.CreatedAt = mustParseTime(createdAt)
	r.UpdatedAt = mustParseTime(updatedAt)
	return &r, nil
}

func (s *Store) scanRecipeRow(rows *sql.Rows) (*Recipe, error) {
	var r Recipe
	var createdAt, updatedAt string
	err := rows.Scan(&r.ID, &r.TeamID, &r.Name, &r.Source, &r.Canvas, &r.CreatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanRecipeRow: %w", err)
	}
	r.CreatedAt = mustParseTime(createdAt)
	r.UpdatedAt = mustParseTime(updatedAt)
	return &r, nil
}
