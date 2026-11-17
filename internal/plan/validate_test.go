package plan_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

func TestValidate_ValidPlan(t *testing.T) {
	t.Parallel()

	err := plan.Validate(validPlan())

	assert.NoError(t, err)
}

func TestValidate_EmptyName(t *testing.T) {
	t.Parallel()

	p := validPlan()
	p.Name = ""

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan name must not be empty")
}

func TestValidate_NoSlices(t *testing.T) {
	t.Parallel()

	p := validPlan()
	p.Slices = nil

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one slice")
}

func TestValidate_SliceRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(p *plan.AppPlan)
		wantErr string
	}{
		{
			name: "empty slice name",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Name = ""
			},
			wantErr: "name must not be empty",
		},
		{
			name: "empty image",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Image = ""
			},
			wantErr: "image must not be empty",
		},
		{
			name: "negative port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = -1
			},
			wantErr: "port is -1 — a port must be between 1 and 65535",
		},
		{
			name: "negative replicas",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Replicas = -3
			},
			wantErr: "replicas must be >= 0",
		},
		{
			name: "zero port is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
			},
			wantErr: "",
		},
		{
			name: "zero replicas is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Replicas = 0
			},
			wantErr: "",
		},
		{
			name: "port exceeds 65535",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 70000
			},
			wantErr: "port is 70000 — a port must be between 1 and 65535",
		},
		{
			name: "invalid slice name format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Name = "My_Slice!"
			},
			wantErr: "DNS-compatible",
		},
		{
			name: "slice name too long",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Name = strings.Repeat("a", 64)
			},
			wantErr: "DNS-compatible",
		},
		{
			name: "image contains whitespace",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Image = "my image:latest"
			},
			wantErr: "must not contain whitespace",
		},
		{
			name: "health path without leading slash",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "healthz"
			},
			wantErr: "must start with /",
		},
		{
			name: "valid health path",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/healthz"
			},
			wantErr: "",
		},
		{
			name: "valid nested health path",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/api/v1/health"
			},
			wantErr: "",
		},
		{
			name: "health path with whitespace",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/health check"
			},
			wantErr: "must start with /",
		},
		{
			name: "health path with shell metacharacters",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/health;echo pwned"
			},
			wantErr: "must start with /",
		},
		{
			name: "health path with newline",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/health\nhttp://evil.com"
			},
			wantErr: "must start with /",
		},
		{
			name: "invalid env key leading digit",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Env = map[string]string{"1BAD": "postgres://localhost/db"}
			},
			wantErr: "env key",
		},
		{
			name: "valid env key",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Env = map[string]string{"DATABASE_URL": "postgres://localhost/db"}
			},
			wantErr: "",
		},
		{
			name: "invalid domain",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Domain = "not a valid domain!"
			},
			wantErr: "is not a valid domain name",
		},
		{
			name: "valid domain",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Domain = "api.example.com"
			},
			wantErr: "",
		},
		{
			name: "invalid restart policy",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].RestartPolicy = "never"
			},
			wantErr: "restart policy",
		},
		{
			name: "valid restart policy",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].RestartPolicy = "always"
			},
			wantErr: "",
		},
		{
			name: "invalid cpu limit format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{CPULimit: "five-hundred"}
			},
			wantErr: "cpu limit",
		},
		{
			name: "valid cpu limit",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{CPULimit: "500m"}
			},
			wantErr: "",
		},
		{
			name: "invalid memory limit format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{MemoryLimit: "two-gigs"}
			},
			wantErr: "memory limit",
		},
		{
			name: "valid memory limit",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{MemoryLimit: "256Mi"}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validPlan()
			tt.modify(p)

			err := plan.Validate(p)

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidate_DuplicateSliceNames(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{Name: "web", Kind: plan.SliceKindWeb, Image: "nginx:latest"},
			{Name: "web", Kind: plan.SliceKindWorker, Image: "worker:latest"},
		},
	}

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate name")
	assert.Contains(t, err.Error(), `"web"`)
}

func TestValidate_NeedsUnknownSlice(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{
				Name:  "web",
				Kind:  plan.SliceKindWeb,
				Image: "nginx:latest",
				Needs: []string{"nonexistent"},
			},
		},
	}

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs unknown slice")
	assert.Contains(t, err.Error(), `"nonexistent"`)
}

func TestValidate_CircularDependency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		slices []plan.Slice
	}{
		{
			name: "direct cycle A->B->A",
			slices: []plan.Slice{
				{Name: "a", Kind: plan.SliceKindWeb, Image: "img:1", Needs: []string{"b"}},
				{Name: "b", Kind: plan.SliceKindWeb, Image: "img:2", Needs: []string{"a"}},
			},
		},
		{
			name: "self reference",
			slices: []plan.Slice{
				{Name: "a", Kind: plan.SliceKindWeb, Image: "img:1", Needs: []string{"a"}},
			},
		},
		{
			name: "three node cycle A->B->C->A",
			slices: []plan.Slice{
				{Name: "a", Kind: plan.SliceKindWeb, Image: "img:1", Needs: []string{"b"}},
				{Name: "b", Kind: plan.SliceKindWeb, Image: "img:2", Needs: []string{"c"}},
				{Name: "c", Kind: plan.SliceKindWeb, Image: "img:3", Needs: []string{"a"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name:   "myapp",
				Slices: tt.slices,
			}

			err := plan.Validate(p)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "circular dependency")
		})
	}
}

func TestValidate_ValidDependencyChain(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{Name: "db", Kind: plan.SliceKindDatabase, Image: "postgres:16"},
			{Name: "cache", Kind: plan.SliceKindCache, Image: "redis:7"},
			{Name: "api", Kind: plan.SliceKindWeb, Image: "api:latest", Needs: []string{"db", "cache"}},
			{Name: "worker", Kind: plan.SliceKindWorker, Image: "worker:latest", Needs: []string{"db"}},
		},
	}

	err := plan.Validate(p)

	assert.NoError(t, err)
}

func TestValidate_MultipleErrorsCollected(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "",
		Slices: []plan.Slice{
			{Name: "", Kind: plan.SliceKindWeb, Image: ""},
		},
	}

	err := plan.Validate(p)

	require.Error(t, err)

	msg := err.Error()
	// Count distinct error messages; expect at least 3:
	// plan name empty, slice name empty, image empty.
	parts := strings.Split(msg, "\n")
	assert.GreaterOrEqual(t, len(parts), 3, "expected at least 3 collected errors, got: %s", msg)
}

func TestValidate_EnvKeyFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "uppercase only", key: "PORT", wantErr: false},
		{name: "uppercase with digits", key: "API_V2", wantErr: false},
		{name: "uppercase with underscores", key: "DATABASE_URL", wantErr: false},
		{name: "single letter", key: "X", wantErr: false},
		{name: "all lowercase", key: "port", wantErr: false},
		{name: "mixed case", key: "Database_Url", wantErr: false},
		{name: "contains hyphen", key: "API-KEY", wantErr: false},
		{name: "starts with underscore", key: "_SECRET", wantErr: false},
		{name: "contains dot", key: "MY_ENV.NAME", wantErr: false},
		{name: "lowercase with dot and hyphen", key: "my.env-name", wantErr: false},
		{name: "ghost nested key", key: "database__connection__user", wantErr: false},
		{name: "gitea nested key", key: "GITEA__database__HOST", wantErr: false},
		{name: "starts with digit", key: "3PO", wantErr: true},
		{name: "contains space", key: "MY VAR", wantErr: true},
		{name: "empty key", key: "", wantErr: true},
		{name: "leading digit with uppercase", key: "1PASSWORD", wantErr: true},
		{name: "leading digit with letters", key: "1BAD", wantErr: true},
		{name: "current directory", key: ".", wantErr: true},
		{name: "parent directory", key: "..", wantErr: true},
		{name: "parent directory prefix", key: "..foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:  "web",
						Kind:  plan.SliceKindWeb,
						Image: "nginx:latest",
						Port:  8080,
						Env:   map[string]string{tt.key: "value"},
					},
				},
			}

			err := plan.Validate(p)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "env key")
				assert.NotContains(t, err.Error(), "^", "the rule must be stated in words, not as a regex")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
