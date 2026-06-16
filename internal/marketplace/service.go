package marketplace

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gshepptech/mozza/internal/store"
	"github.com/gshepptech/mozza/internal/template"
)

// Service provides recipe marketplace operations: search, get, install, and
// deploy. It combines the embedded template catalog with an optional remote
// index fetched from GitHub.
type Service struct {
	catalog  *template.Catalog
	store    *store.Store
	indexURL string
	cache    *indexCache
}

// Option configures a Service.
type Option func(*Service)

// WithIndexURL overrides the default remote index URL.
func WithIndexURL(url string) Option {
	return func(s *Service) {
		s.indexURL = url
	}
}

// New creates a marketplace Service backed by the embedded catalog and store.
func New(catalog *template.Catalog, st *store.Store, opts ...Option) *Service {
	s := &Service{
		catalog:  catalog,
		store:    st,
		indexURL: defaultIndexURL,
		cache:    &indexCache{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ListParams holds parameters for listing/searching recipes.
type ListParams struct {
	Query    string
	Category string
	Tags     []string
	Page     int
	PerPage  int
}

// ListResult is a paginated list of recipes.
type ListResult struct {
	Recipes    []SearchResult `json:"recipes"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
}

// Search returns recipes matching the given parameters with pagination.
func (s *Service) Search(ctx context.Context, params ListParams) (*ListResult, error) {
	params = normalizeParams(params)

	all := s.allTemplates(ctx)
	results := searchTemplates(all, params.Query, params.Category, params.Tags)

	total := len(results)
	totalPages := (total + params.PerPage - 1) / params.PerPage
	if totalPages == 0 {
		totalPages = 1
	}

	start := (params.Page - 1) * params.PerPage
	if start >= total {
		return &ListResult{
			Recipes:    []SearchResult{},
			Total:      total,
			Page:       params.Page,
			PerPage:    params.PerPage,
			TotalPages: totalPages,
		}, nil
	}

	end := start + params.PerPage
	if end > total {
		end = total
	}

	return &ListResult{
		Recipes:    results[start:end],
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

// Get returns a single recipe by name (case-insensitive).
func (s *Service) Get(ctx context.Context, name string) (*template.Template, error) {
	all := s.allTemplates(ctx)
	for _, t := range all {
		if t.ID == name || t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("recipe %q not found", name)
}

// Install returns the .mozza source content for a recipe.
func (s *Service) Install(ctx context.Context, name string) (string, error) {
	t, err := s.Get(ctx, name)
	if err != nil {
		return "", err
	}

	if t.Source == "" {
		return "", fmt.Errorf("recipe %q has no source content", name)
	}

	// Resolve template variables with their defaults so the installed
	// recipe file is immediately usable by "mozza up". Required variables
	// without defaults are left as {{VAR}} for the user to fill in.
	rendered := template.RenderDefaults(*t)

	return rendered, nil
}

// Categories returns all unique category names across all sources.
func (s *Service) Categories(ctx context.Context) []string {
	all := s.allTemplates(ctx)
	seen := make(map[string]bool)
	for _, t := range all {
		if t.Category != "" {
			seen[t.Category] = true
		}
	}

	cats := make([]string, 0, len(seen))
	for cat := range seen {
		cats = append(cats, cat)
	}
	return cats
}

// Refresh forces a re-fetch of the remote index.
func (s *Service) Refresh(ctx context.Context) error {
	entries, err := fetchIndex(ctx, s.indexURL)
	if err != nil {
		slog.Warn("marketplace: remote index fetch failed, using embedded catalog only",
			"error", err)
		return err
	}

	s.cache.set(entries)

	if s.store != nil {
		syncToStore(ctx, s.store, entries)
	}

	slog.Info("marketplace: refreshed remote index", "count", len(entries))
	return nil
}

// allTemplates merges embedded catalog templates with remote index entries.
// Embedded templates take precedence over remote entries with the same ID.
func (s *Service) allTemplates(ctx context.Context) []template.Template {
	var all []template.Template
	seen := make(map[string]bool)

	// Embedded catalog first (highest priority).
	if s.catalog != nil {
		for _, t := range s.catalog.List("") {
			seen[t.ID] = true
			all = append(all, t)
		}
	}

	// Merge remote index entries (if cached and not stale).
	if !s.cache.isStale() {
		for _, e := range s.cache.get() {
			if !seen[e.ID] {
				seen[e.ID] = true
				all = append(all, entryToTemplate(e))
			}
		}
	}

	return all
}

// normalizeParams applies defaults and caps to pagination parameters.
func normalizeParams(p ListParams) ListParams {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 20
	}
	if p.PerPage > 100 {
		p.PerPage = 100
	}
	return p
}
