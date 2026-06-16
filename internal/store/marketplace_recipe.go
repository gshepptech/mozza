package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MarketplaceRecipe represents a cached recipe from the marketplace.
type MarketplaceRecipe struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Category      string     `json:"category,omitempty"`
	Tags          string     `json:"tags,omitempty"`
	ContentHash   string     `json:"content_hash,omitempty"`
	RecipeContent string     `json:"recipe_content,omitempty"`
	FetchedAt     *time.Time `json:"fetched_at,omitempty"`
}

// CreateMarketplaceRecipe inserts a new marketplace recipe cache entry.
func (s *Store) CreateMarketplaceRecipe(ctx context.Context, name, category, tags, contentHash, recipeContent string) (*MarketplaceRecipe, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO marketplace_cache (name, category, tags, content_hash, recipe_content, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		name, nullableString(category), nullableString(tags),
		nullableString(contentHash), nullableString(recipeContent), now,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateMarketplaceRecipe: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("CreateMarketplaceRecipe: last insert id: %w", err)
	}

	ts := mustParseTime(now)
	return &MarketplaceRecipe{
		ID:            id,
		Name:          name,
		Category:      category,
		Tags:          tags,
		ContentHash:   contentHash,
		RecipeContent: recipeContent,
		FetchedAt:     &ts,
	}, nil
}

// GetMarketplaceRecipe returns a marketplace recipe by name.
func (s *Store) GetMarketplaceRecipe(ctx context.Context, name string) (*MarketplaceRecipe, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, category, tags, content_hash, recipe_content, fetched_at
		 FROM marketplace_cache WHERE name = ?`, name,
	)
	return scanMarketplaceRecipe(row)
}

// ListMarketplaceRecipes returns all cached marketplace recipes.
func (s *Store) ListMarketplaceRecipes(ctx context.Context) ([]MarketplaceRecipe, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, category, tags, content_hash, recipe_content, fetched_at
		 FROM marketplace_cache ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListMarketplaceRecipes: %w", err)
	}
	defer rows.Close()

	var recipes []MarketplaceRecipe
	for rows.Next() {
		r, err := scanMarketplaceRecipeRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListMarketplaceRecipes: %w", err)
		}
		recipes = append(recipes, *r)
	}
	return recipes, rows.Err()
}

// SearchMarketplaceRecipes searches recipes by name or category.
func (s *Store) SearchMarketplaceRecipes(ctx context.Context, query string) ([]MarketplaceRecipe, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, category, tags, content_hash, recipe_content, fetched_at
		 FROM marketplace_cache
		 WHERE name LIKE ? OR category LIKE ? OR tags LIKE ?
		 ORDER BY name ASC`,
		pattern, pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("SearchMarketplaceRecipes: %w", err)
	}
	defer rows.Close()

	var recipes []MarketplaceRecipe
	for rows.Next() {
		r, err := scanMarketplaceRecipeRow(rows)
		if err != nil {
			return nil, fmt.Errorf("SearchMarketplaceRecipes: %w", err)
		}
		recipes = append(recipes, *r)
	}
	return recipes, rows.Err()
}

// UpsertMarketplaceRecipe inserts or updates a marketplace recipe by name.
func (s *Store) UpsertMarketplaceRecipe(ctx context.Context, name, category, tags, contentHash, recipeContent string) (*MarketplaceRecipe, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Try update first.
	res, err := s.db.ExecContext(ctx,
		`UPDATE marketplace_cache
		 SET category = ?, tags = ?, content_hash = ?, recipe_content = ?, fetched_at = ?
		 WHERE name = ?`,
		nullableString(category), nullableString(tags),
		nullableString(contentHash), nullableString(recipeContent), now, name,
	)
	if err != nil {
		return nil, fmt.Errorf("UpsertMarketplaceRecipe: update: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("UpsertMarketplaceRecipe: rows affected: %w", err)
	}

	if n == 0 {
		return s.CreateMarketplaceRecipe(ctx, name, category, tags, contentHash, recipeContent)
	}

	return s.GetMarketplaceRecipe(ctx, name)
}

func scanMarketplaceRecipe(row *sql.Row) (*MarketplaceRecipe, error) {
	var r MarketplaceRecipe
	var category, tags, contentHash, recipeContent, fetchedAt sql.NullString

	err := row.Scan(&r.ID, &r.Name, &category, &tags, &contentHash, &recipeContent, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanMarketplaceRecipe: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanMarketplaceRecipe: %w", err)
	}

	if category.Valid {
		r.Category = category.String
	}
	if tags.Valid {
		r.Tags = tags.String
	}
	if contentHash.Valid {
		r.ContentHash = contentHash.String
	}
	if recipeContent.Valid {
		r.RecipeContent = recipeContent.String
	}
	if fetchedAt.Valid {
		t := mustParseTime(fetchedAt.String)
		r.FetchedAt = &t
	}
	return &r, nil
}

func scanMarketplaceRecipeRow(rows *sql.Rows) (*MarketplaceRecipe, error) {
	var r MarketplaceRecipe
	var category, tags, contentHash, recipeContent, fetchedAt sql.NullString

	err := rows.Scan(&r.ID, &r.Name, &category, &tags, &contentHash, &recipeContent, &fetchedAt)
	if err != nil {
		return nil, fmt.Errorf("scanMarketplaceRecipeRow: %w", err)
	}

	if category.Valid {
		r.Category = category.String
	}
	if tags.Valid {
		r.Tags = tags.String
	}
	if contentHash.Valid {
		r.ContentHash = contentHash.String
	}
	if recipeContent.Valid {
		r.RecipeContent = recipeContent.String
	}
	if fetchedAt.Valid {
		t := mustParseTime(fetchedAt.String)
		r.FetchedAt = &t
	}
	return &r, nil
}
