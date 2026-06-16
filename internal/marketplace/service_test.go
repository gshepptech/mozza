package marketplace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/template"
)

func testCatalog(t *testing.T) *template.Catalog {
	t.Helper()
	cat, err := template.Load()
	require.NoError(t, err)
	return cat
}

func TestNew(t *testing.T) {
	cat := testCatalog(t)
	svc := New(cat, nil)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.cache)
}

func TestSearch_AllRecipes(t *testing.T) {
	svc := New(testCatalog(t), nil)

	result, err := svc.Search(context.Background(), ListParams{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, 15)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PerPage)
}

func TestSearch_ByQuery(t *testing.T) {
	svc := New(testCatalog(t), nil)

	result, err := svc.Search(context.Background(), ListParams{Query: "wordpress"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Recipes), 1)
	assert.Equal(t, "WordPress", result.Recipes[0].Template.Name)
}

func TestSearch_ByCategory(t *testing.T) {
	svc := New(testCatalog(t), nil)

	result, err := svc.Search(context.Background(), ListParams{Category: "monitoring"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Recipes), 2)
	for _, r := range result.Recipes {
		assert.Equal(t, "monitoring", r.Template.Category)
	}
}

func TestSearch_ByTags(t *testing.T) {
	svc := New(testCatalog(t), nil)

	result, err := svc.Search(context.Background(), ListParams{Tags: []string{"blog"}})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Recipes), 1)
}

func TestSearch_Pagination(t *testing.T) {
	svc := New(testCatalog(t), nil)

	result, err := svc.Search(context.Background(), ListParams{PerPage: 3, Page: 1})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Recipes), 3)
	assert.GreaterOrEqual(t, result.TotalPages, 2)

	// Page 2 should also work.
	result2, err := svc.Search(context.Background(), ListParams{PerPage: 3, Page: 2})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result2.Recipes), 1)
}

func TestSearch_PaginationCap(t *testing.T) {
	svc := New(testCatalog(t), nil)

	// Per page over 100 should be capped.
	result, err := svc.Search(context.Background(), ListParams{PerPage: 200})
	require.NoError(t, err)
	assert.Equal(t, 100, result.PerPage)
}

func TestSearch_EmptyPage(t *testing.T) {
	svc := New(testCatalog(t), nil)

	result, err := svc.Search(context.Background(), ListParams{Page: 999})
	require.NoError(t, err)
	assert.Empty(t, result.Recipes)
}

func TestGet_Found(t *testing.T) {
	svc := New(testCatalog(t), nil)

	tmpl, err := svc.Get(context.Background(), "wordpress")
	require.NoError(t, err)
	assert.Equal(t, "WordPress", tmpl.Name)
	assert.NotEmpty(t, tmpl.Source)
}

func TestGet_NotFound(t *testing.T) {
	svc := New(testCatalog(t), nil)

	_, err := svc.Get(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInstall_Success(t *testing.T) {
	svc := New(testCatalog(t), nil)

	source, err := svc.Install(context.Background(), "wordpress")
	require.NoError(t, err)
	assert.Contains(t, source, "wordpress")
}

func TestInstall_NotFound(t *testing.T) {
	svc := New(testCatalog(t), nil)

	_, err := svc.Install(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestCategories(t *testing.T) {
	svc := New(testCatalog(t), nil)

	cats := svc.Categories(context.Background())
	assert.GreaterOrEqual(t, len(cats), 5)
}

func TestNormalizeParams(t *testing.T) {
	tests := []struct {
		name   string
		input  ListParams
		wantPg int
		wantPP int
	}{
		{"defaults", ListParams{}, 1, 20},
		{"negative page", ListParams{Page: -1}, 1, 20},
		{"zero per_page", ListParams{PerPage: 0}, 1, 20},
		{"over max", ListParams{PerPage: 200}, 1, 100},
		{"valid", ListParams{Page: 3, PerPage: 50}, 3, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeParams(tt.input)
			assert.Equal(t, tt.wantPg, got.Page)
			assert.Equal(t, tt.wantPP, got.PerPage)
		})
	}
}
