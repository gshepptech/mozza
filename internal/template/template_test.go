package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/gshepptech/mozza/internal/recipe"
)

func TestRenderRecipe_Substitution(t *testing.T) {
	tmpl := Template{
		Source: `set WORDPRESS_DB_PASSWORD to "{{DB_PASSWORD}}"`,
		Variables: []TemplateVar{
			{Key: "DB_PASSWORD", Required: true},
		},
	}

	result, err := RenderRecipe(tmpl, map[string]string{
		"DB_PASSWORD": "s3cret",
	})

	require.NoError(t, err)
	assert.Equal(t, `set WORDPRESS_DB_PASSWORD to "s3cret"`, result)
}

func TestRenderRecipe_MissingRequired(t *testing.T) {
	tmpl := Template{
		Source: `set PASSWORD to "{{PASSWORD}}"`,
		Variables: []TemplateVar{
			{Key: "PASSWORD", Required: true},
		},
	}

	_, err := RenderRecipe(tmpl, map[string]string{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "PASSWORD")
	assert.Contains(t, err.Error(), "required")
}

func TestRenderRecipe_DefaultValue(t *testing.T) {
	tmpl := Template{
		Source: `mysql 8, {{STORAGE_SIZE}}`,
		Variables: []TemplateVar{
			{Key: "STORAGE_SIZE", Default: "10Gi", Required: false},
		},
	}

	result, err := RenderRecipe(tmpl, map[string]string{})

	require.NoError(t, err)
	assert.Equal(t, `mysql 8, 10Gi`, result)
}

func TestCatalogLoad(t *testing.T) {
	catalog, err := Load()
	require.NoError(t, err)

	templates := catalog.List("")
	assert.Len(t, templates, 15)
}

func TestCatalogGet(t *testing.T) {
	catalog, err := Load()
	require.NoError(t, err)

	tmpl, err := catalog.Get("wordpress")
	require.NoError(t, err)
	assert.Equal(t, "WordPress", tmpl.Name)
	assert.Equal(t, "cms", tmpl.Category)
	assert.True(t, tmpl.Official)
	assert.NotEmpty(t, tmpl.Source)
}

func TestCatalogGet_NotFound(t *testing.T) {
	catalog, err := Load()
	require.NoError(t, err)

	_, err = catalog.Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCatalogList_FilterByCategory(t *testing.T) {
	catalog, err := Load()
	require.NoError(t, err)

	cms := catalog.List("cms")
	assert.Len(t, cms, 3, "expected wordpress, ghost, and nextcloud in cms category")

	devtools := catalog.List("devtools")
	assert.Len(t, devtools, 2, "expected gitea and redis-commander in devtools category")
}

func TestCatalogCategories(t *testing.T) {
	catalog, err := Load()
	require.NoError(t, err)

	cats := catalog.Categories()
	assert.GreaterOrEqual(t, len(cats), 6)
	assert.Contains(t, cats, "cms")
	assert.Contains(t, cats, "devtools")
	assert.Contains(t, cats, "monitoring")
	assert.Contains(t, cats, "databases")
}

func TestAllTemplatesParseAfterRender(t *testing.T) {
	catalog, err := Load()
	require.NoError(t, err)

	// Sample values for each template's required variables.
	sampleVars := map[string]map[string]string{
		"wordpress":       {"DB_PASSWORD": "testpass", "STORAGE_SIZE": "10Gi"},
		"ghost":           {"DB_PASSWORD": "testpass", "DOMAIN": "localhost"},
		"gitea":           {"DB_PASSWORD": "testpass"},
		"uptime-kuma":     {},
		"plausible":       {"DB_PASSWORD": "testpass", "SECRET_KEY": "abc123secret", "DOMAIN": "localhost"},
		"minio":           {"ACCESS_KEY": "minioadmin", "SECRET_KEY": "miniosecret"},
		"postgres-admin":  {"DB_PASSWORD": "testpass", "DB_USER": "postgres", "DB_NAME": "appdb", "ADMIN_EMAIL": "a@b.com", "STORAGE_SIZE": "10Gi"},
		"n8n":             {"DB_PASSWORD": "testpass"},
		"redis-commander": {},
		"hobbyfarm":       {"GARGANTUA_VERSION": "v3.3.5", "UI_VERSION": "v3.3.6", "ADMIN_VERSION": "v3.3.6"},
		"grafana":         {"ADMIN_PASSWORD": "testpass"},
		"rabbitmq":        {"RABBITMQ_PASSWORD": "testpass"},
		"nextcloud":       {"DB_PASSWORD": "testpass", "ADMIN_PASSWORD": "testpass"},
		"prometheus":      {},
		"mattermost":      {"DB_PASSWORD": "testpass"},
	}

	templates := catalog.List("")
	for _, tmpl := range templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			vars, ok := sampleVars[tmpl.ID]
			require.True(t, ok, "missing sample vars for template %s", tmpl.ID)

			rendered, err := RenderRecipe(tmpl, vars)
			require.NoError(t, err, "RenderRecipe failed for %s", tmpl.ID)

			p := recipe.NewParser(rendered)
			r, err := p.Parse()
			require.NoError(t, err, "Parse failed for %s:\n%s", tmpl.ID, rendered)
			assert.NotEmpty(t, r.Name, "recipe Name should be set for %s", tmpl.ID)
		})
	}
}
