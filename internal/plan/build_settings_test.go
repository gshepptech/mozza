package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/recipe"
)

func TestBuild_ResourceSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cpuLimit      string
		memoryLimit   string
		wantResources *ResourceSpec
	}{
		{
			name:        "both cpu and memory",
			cpuLimit:    "500m",
			memoryLimit: "256Mi",
			wantResources: &ResourceSpec{
				CPULimit:    "500m",
				MemoryLimit: "256Mi",
			},
		},
		{
			name:     "cpu only",
			cpuLimit: "1",
			wantResources: &ResourceSpec{
				CPULimit: "1",
			},
		},
		{
			name:        "memory only",
			memoryLimit: "1Gi",
			wantResources: &ResourceSpec{
				MemoryLimit: "1Gi",
			},
		},
		{
			name:          "no resource limits",
			wantResources: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "resource-test",
				Slices: []recipe.Slice{
					{
						Name:        "api",
						Image:       "api:latest",
						Port:        8080,
						CPULimit:    tt.cpuLimit,
						MemoryLimit: tt.memoryLimit,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantResources, plan.Slices[0].Resources)
		})
	}
}

func TestBuild_RestartPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policy     string
		wantPolicy string
	}{
		{"always", "always", "always"},
		{"unless-stopped", "unless-stopped", "unless-stopped"},
		{"on-failure", "on-failure", "on-failure"},
		{"no", "no", "no"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "restart-test",
				Slices: []recipe.Slice{
					{
						Name:          "api",
						Image:         "api:latest",
						Port:          8080,
						RestartPolicy: tt.policy,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantPolicy, plan.Slices[0].RestartPolicy)
		})
	}
}

func TestBuild_Domain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		domain     string
		wantDomain string
	}{
		{"simple domain", "example.com", "example.com"},
		{"subdomain", "api.example.com", "api.example.com"},
		{"deep subdomain", "v1.api.example.com", "v1.api.example.com"},
		{"empty domain", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "domain-test",
				Slices: []recipe.Slice{
					{
						Name:   "api",
						Image:  "api:latest",
						Port:   8080,
						Domain: tt.domain,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantDomain, plan.Slices[0].Domain)
		})
	}
}

func TestBuild_Namespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		namespace     string
		wantNamespace string
	}{
		{"production", "production", "production"},
		{"staging", "staging", "staging"},
		{"custom", "my-namespace", "my-namespace"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name:      "ns-test",
				Namespace: tt.namespace,
				Slices: []recipe.Slice{
					{
						Name:  "api",
						Image: "api:latest",
						Port:  8080,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)

			assert.Equal(t, tt.wantNamespace, plan.Namespace)
		})
	}
}
