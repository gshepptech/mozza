package plan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
)

func TestValidate_Quantities(t *testing.T) {
	t.Parallel()

	// Helpers keep each row to the one value it is about. Each sets the kind the
	// field belongs to, so the compile check below exercises the resource.MustParse
	// call this rule exists to keep unreachable.
	stateful := func(size string) func(*plan.AppPlan) {
		return func(p *plan.AppPlan) {
			p.Slices[0].Name = "queue"
			p.Slices[0].Kind = plan.SliceKindStateful
			p.Slices[0].StatefulStorage = size
			p.Slices[0].OrderedStartup = true
		}
	}
	database := func(size string) func(*plan.AppPlan) {
		return func(p *plan.AppPlan) {
			p.Slices[0].Name = "db"
			p.Slices[0].Kind = plan.SliceKindDatabase
			p.Slices[0].Image = "postgres:16"
			p.Slices[0].Database = &plan.DatabaseSpec{Storage: size}
		}
	}
	cache := func(size string) func(*plan.AppPlan) {
		return func(p *plan.AppPlan) {
			p.Slices[0].Name = "cache"
			p.Slices[0].Kind = plan.SliceKindCache
			p.Slices[0].Image = "redis:7"
			p.Slices[0].Cache = &plan.CacheSpec{Storage: size}
		}
	}
	cpu := func(v string) func(*plan.AppPlan) {
		return func(p *plan.AppPlan) { p.Slices[0].Resources = &plan.ResourceSpec{CPULimit: v} }
	}
	memory := func(v string) func(*plan.AppPlan) {
		return func(p *plan.AppPlan) { p.Slices[0].Resources = &plan.ResourceSpec{MemoryLimit: v} }
	}

	tests := []struct {
		name    string
		modify  func(p *plan.AppPlan)
		wantErr string
	}{
		// Every field rejects a value that is not a quantity, so all five are
		// proved wired rather than assumed.
		{
			name:    "per-copy storage is not a quantity",
			modify:  stateful("BIGGISH"),
			wantErr: `slice[0] "queue": per-copy storage "BIGGISH" is not a quantity Kubernetes understands`,
		},
		{
			name:    "database storage is not a quantity",
			modify:  database("BIGGISH"),
			wantErr: `slice[0] "db": database storage "BIGGISH" is not a quantity Kubernetes understands`,
		},
		{
			name:    "cache storage is not a quantity",
			modify:  cache("BIGGISH"),
			wantErr: `slice[0] "cache": cache storage "BIGGISH" is not a quantity Kubernetes understands`,
		},
		{
			name:    "cpu limit is not a quantity",
			modify:  cpu("BIGGISH"),
			wantErr: `slice[0] "web": cpu limit "BIGGISH" is not a quantity Kubernetes understands`,
		},
		{
			name:    "memory limit is not a quantity",
			modify:  memory("BIGGISH"),
			wantErr: `slice[0] "web": memory limit "BIGGISH" is not a quantity Kubernetes understands`,
		},

		// Only lowercase k is a decimal SI suffix, so 500K is a realistic typo
		// that would otherwise reach MustParse.
		{
			name:    "capital K is not a suffix Kubernetes knows",
			modify:  database("500K"),
			wantErr: `slice[0] "db": database storage "500K" is not a quantity Kubernetes understands`,
		},
		{
			name:    "a quantity with a space in it",
			modify:  database("20 GB"),
			wantErr: `slice[0] "db": database storage "20 GB" is not a quantity Kubernetes understands`,
		},

		// An unrendered placeholder is told what is wrong with it, not handed a
		// number-format complaint.
		{
			name:    "unrendered placeholder in database storage",
			modify:  database("{{STORAGE_SIZE}}"),
			wantErr: `slice[0] "db": database storage "{{STORAGE_SIZE}}" is still a template variable — substitute a real value before deploying`,
		},
		{
			name:    "unrendered placeholder in per-copy storage",
			modify:  stateful("{{STORAGE_SIZE}}"),
			wantErr: `slice[0] "queue": per-copy storage "{{STORAGE_SIZE}}" is still a template variable — substitute a real value before deploying`,
		},
		{
			name:    "unrendered placeholder in a memory limit",
			modify:  memory("{{MEMORY_LIMIT}}"),
			wantErr: `slice[0] "web": memory limit "{{MEMORY_LIMIT}}" is still a template variable — substitute a real value before deploying`,
		},

		// Negative validates and compiles today, emitting storage: -1Gi.
		{
			name:    "negative storage",
			modify:  database("-1Gi"),
			wantErr: `slice[0] "db": database storage "-1Gi" is negative — a quantity must be zero or more`,
		},
		{
			name:    "negative memory limit",
			modify:  memory("-1Gi"),
			wantErr: `slice[0] "web": memory limit "-1Gi" is negative — a quantity must be zero or more`,
		},

		{name: "storage in binary units", modify: database("10Gi"), wantErr: ""},
		{name: "cache storage in binary units", modify: cache("512Mi"), wantErr: ""},
		{name: "per-copy storage in binary units", modify: stateful("10Gi"), wantErr: ""},
		{name: "cpu limit in millis", modify: cpu("500m"), wantErr: ""},
		{name: "memory limit in binary units", modify: memory("256Mi"), wantErr: ""},

		// Each of these is a quantity Kubernetes accepts and the deleted regex
		// rejected, so they prove the rule is the platform's rather than a
		// restatement of it.
		{name: "decimal gigabytes", modify: memory("2G"), wantErr: ""},
		{name: "exponent notation", modify: memory("1e3"), wantErr: ""},
		{name: "exbibytes", modify: memory("1Ei"), wantErr: ""},
		{name: "lowercase decimal kilo", modify: database("500k"), wantErr: ""},
		{name: "micro-units as a cpu limit", modify: cpu("500u"), wantErr: ""},

		// Zero is Kubernetes' own rule: non-negative, not positive.
		{name: "zero storage", modify: database("0"), wantErr: ""},
		{name: "zero cpu limit", modify: cpu("0"), wantErr: ""},

		// Empty is the "no volume" sentinel every storage-free recipe relies on.
		{name: "empty storage means no volume", modify: database(""), wantErr: ""},
		{name: "empty cache storage means no volume", modify: cache(""), wantErr: ""},
		{name: "empty per-copy storage means no volume", modify: stateful(""), wantErr: ""},
		{name: "empty resource limits", modify: memory(""), wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validPlan()
			tt.modify(p)

			err := plan.Validate(p)

			if tt.wantErr == "" {
				require.NoError(t, err)

				// Criterion 2, mechanically: anything that validates must survive
				// the compiler, whose quantity parsing panics rather than erroring.
				assert.NotPanics(t, func() { _, _ = k8s.New().CompileObjects(p) })

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.NotContains(t, err.Error(), "^", "the rule must be stated in words, not as a regex")
		})
	}
}
