package marketplace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gshepptech/mozza/internal/template"
)

func sampleTemplates() []template.Template {
	return []template.Template{
		{ID: "wordpress", Name: "WordPress", Description: "Blog CMS", Category: "cms", Tags: []string{"blog", "php"}},
		{ID: "grafana", Name: "Grafana", Description: "Dashboards", Category: "monitoring", Tags: []string{"monitoring", "dashboards"}},
		{ID: "postgres-admin", Name: "Postgres + pgAdmin", Description: "Database with UI", Category: "databases", Tags: []string{"database", "postgres"}},
		{ID: "rabbitmq", Name: "RabbitMQ", Description: "Message broker", Category: "communication", Tags: []string{"messaging", "queue"}},
	}
}

func TestFuzzyScore_ExactMatch(t *testing.T) {
	tmpl := template.Template{ID: "wordpress", Name: "WordPress"}
	assert.Equal(t, 1.0, fuzzyScore(tmpl, "wordpress"))
	assert.Equal(t, 1.0, fuzzyScore(tmpl, "WordPress"))
}

func TestFuzzyScore_PartialName(t *testing.T) {
	tmpl := template.Template{ID: "wordpress", Name: "WordPress"}
	assert.Equal(t, 0.8, fuzzyScore(tmpl, "word"))
}

func TestFuzzyScore_DescriptionMatch(t *testing.T) {
	tmpl := template.Template{ID: "wp", Name: "WP", Description: "A blog platform"}
	assert.Equal(t, 0.6, fuzzyScore(tmpl, "blog"))
}

func TestFuzzyScore_TagMatch(t *testing.T) {
	tmpl := template.Template{ID: "wp", Name: "WP", Tags: []string{"blog", "cms"}}
	assert.Equal(t, 0.7, fuzzyScore(tmpl, "blog"))
}

func TestFuzzyScore_NoMatch(t *testing.T) {
	tmpl := template.Template{ID: "wp", Name: "WP", Description: "A CMS"}
	assert.Equal(t, 0.0, fuzzyScore(tmpl, "kubernetes"))
}

func TestSearchTemplates_NoFilters(t *testing.T) {
	results := searchTemplates(sampleTemplates(), "", "", nil)
	assert.Len(t, results, 4)
}

func TestSearchTemplates_CategoryFilter(t *testing.T) {
	results := searchTemplates(sampleTemplates(), "", "cms", nil)
	assert.Len(t, results, 1)
	assert.Equal(t, "wordpress", results[0].Template.ID)
}

func TestSearchTemplates_TagFilter(t *testing.T) {
	results := searchTemplates(sampleTemplates(), "", "", []string{"database"})
	assert.Len(t, results, 1)
	assert.Equal(t, "postgres-admin", results[0].Template.ID)
}

func TestSearchTemplates_QueryAndCategory(t *testing.T) {
	results := searchTemplates(sampleTemplates(), "dash", "monitoring", nil)
	assert.Len(t, results, 1)
	assert.Equal(t, "grafana", results[0].Template.ID)
}

func TestSearchTemplates_NoResults(t *testing.T) {
	results := searchTemplates(sampleTemplates(), "nonexistent", "", nil)
	assert.Empty(t, results)
}

func TestHasAllTags(t *testing.T) {
	assert.True(t, hasAllTags([]string{"a", "b", "c"}, []string{"a", "b"}))
	assert.False(t, hasAllTags([]string{"a", "b"}, []string{"a", "c"}))
	assert.True(t, hasAllTags([]string{"Blog"}, []string{"blog"}))
}

func TestSearchTemplates_SortedByScore(t *testing.T) {
	templates := []template.Template{
		{ID: "other", Name: "Other App", Description: "Something about postgres"},
		{ID: "postgres-admin", Name: "Postgres + pgAdmin", Description: "Database with UI", Tags: []string{"postgres"}},
	}
	results := searchTemplates(templates, "postgres", "", nil)
	assert.GreaterOrEqual(t, len(results), 2)
	assert.GreaterOrEqual(t, results[0].Score, results[1].Score)
}
