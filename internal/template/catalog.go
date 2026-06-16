package template

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Catalog holds all available templates.
type Catalog struct {
	templates []Template
}

// Load creates a Catalog from the embedded filesystem.
func Load() (*Catalog, error) {
	data, err := recipesFS.ReadFile("recipes/catalog.json")
	if err != nil {
		return nil, fmt.Errorf("read catalog.json: %w", err)
	}

	var templates []Template
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, fmt.Errorf("parse catalog.json: %w", err)
	}

	for i := range templates {
		filename := "recipes/" + templates[i].ID + ".mozza"
		src, err := recipesFS.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", templates[i].ID, err)
		}
		templates[i].Source = string(src)
	}

	return &Catalog{templates: templates}, nil
}

// List returns templates, optionally filtered by category.
// Pass an empty string to return all templates.
func (c *Catalog) List(category string) []Template {
	if category == "" {
		out := make([]Template, len(c.templates))
		copy(out, c.templates)
		return out
	}

	var out []Template
	for _, t := range c.templates {
		if t.Category == category {
			out = append(out, t)
		}
	}
	return out
}

// Get returns a single template by ID.
func (c *Catalog) Get(id string) (*Template, error) {
	for _, t := range c.templates {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template %q not found", id)
}

// Categories returns all unique category names, sorted alphabetically.
func (c *Catalog) Categories() []string {
	seen := make(map[string]bool)
	for _, t := range c.templates {
		seen[t.Category] = true
	}

	cats := make([]string, 0, len(seen))
	for cat := range seen {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}
