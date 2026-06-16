// Package marketplace provides recipe discovery, search, and installation
// on top of the embedded template catalog and optional remote index.
package marketplace

import (
	"strings"

	"github.com/gshepptech/mozza/internal/template"
)

// SearchResult wraps a template with a relevance score.
type SearchResult struct {
	Template template.Template `json:"template"`
	Score    float64           `json:"score"`
}

// searchTemplates performs fuzzy name search combined with exact category and
// tag filtering against the provided templates.
func searchTemplates(templates []template.Template, query, category string, tags []string) []SearchResult {
	var results []SearchResult

	for _, t := range templates {
		// Exact category filter.
		if category != "" && !strings.EqualFold(t.Category, category) {
			continue
		}

		// Exact tag filter — all requested tags must be present.
		if len(tags) > 0 && !hasAllTags(t.Tags, tags) {
			continue
		}

		// Fuzzy name/description match when a query is provided.
		score := 1.0
		if query != "" {
			score = fuzzyScore(t, query)
			if score == 0 {
				continue
			}
		}

		results = append(results, SearchResult{Template: t, Score: score})
	}

	// Sort by score descending (simple insertion sort for small lists).
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	return results
}

// fuzzyScore returns a relevance score (0-1) for the template against the query.
// Returns 0 if there is no match.
func fuzzyScore(t template.Template, query string) float64 {
	q := strings.ToLower(query)
	name := strings.ToLower(t.Name)
	desc := strings.ToLower(t.Description)
	id := strings.ToLower(t.ID)

	// Exact name match is highest score.
	if name == q || id == q {
		return 1.0
	}

	// Name contains query.
	if strings.Contains(name, q) || strings.Contains(id, q) {
		return 0.8
	}

	// Description contains query.
	if strings.Contains(desc, q) {
		return 0.6
	}

	// Tag match.
	for _, tag := range t.Tags {
		if strings.EqualFold(tag, q) {
			return 0.7
		}
		if strings.Contains(strings.ToLower(tag), q) {
			return 0.5
		}
	}

	// Category match.
	if strings.Contains(strings.ToLower(t.Category), q) {
		return 0.4
	}

	return 0
}

// hasAllTags returns true if the template tags contain all of the requested tags
// (case-insensitive).
func hasAllTags(templateTags, requestedTags []string) bool {
	tagSet := make(map[string]bool, len(templateTags))
	for _, t := range templateTags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, rt := range requestedTags {
		if !tagSet[strings.ToLower(rt)] {
			return false
		}
	}
	return true
}
