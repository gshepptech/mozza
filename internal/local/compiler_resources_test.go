package local_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
)

// TestCompile_ResourceLimitConversion pins both quantity converters. They are
// the only real arithmetic in this package, and the formatter picks a unit
// rather than echoing the recipe, so an off-by-one shift would produce a
// plausible-looking file that means something else.
//
// Every wantMemory below was produced by writing it into a compose file and
// running `docker compose config` (Compose v5.1.2); the byte count in each
// comment is what Compose reported, not what the shift arithmetic predicts.
// Compose's K/M/G are 1024-based, so 256M is exactly 256Mi, and it rejects the
// "i" forms outright.
func TestCompile_ResourceLimitConversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cpu             string
		memory          string
		wantCPU         string
		wantMemory      string
		wantNoResources bool
	}{
		// Unchanged by this conversion: the values recipes actually carry.
		{name: "mebibytes keep their size", memory: "256Mi", wantMemory: "256M"}, // 268435456
		{name: "gibibytes keep their size", memory: "1Gi", wantMemory: "1G"},     // 1073741824
		{name: "kibibytes keep their size", memory: "1Ki", wantMemory: "1K"},     // 1024
		{name: "millis become a cpu fraction", cpu: "500m", wantCPU: "0.5"},      // cpus: 0.5
		{name: "a whole cpu passes through", cpu: "2", wantCPU: "2"},             // cpus: 2

		// Decimal SI: Kubernetes means powers of ten, Compose reads powers of
		// two, so the value has to be restated in Compose's units to survive.
		{name: "decimal gigabytes are restated", memory: "2G", wantMemory: "1953125K"}, // 2000000000, unconverted: 2147483648
		{name: "decimal kilobytes are restated", memory: "500k", wantMemory: "500000"}, // 500000, unconverted: 512000

		// Compose takes no "i" suffix and no "u", so these must be converted or
		// the file fails to load.
		{name: "tebibytes are converted", memory: "1Ti", wantMemory: "1024G"},       // 1099511627776, unconverted: rejected
		{name: "exbibytes are converted", memory: "1Ei", wantMemory: "1073741824G"}, // 1152921504606846976, unconverted: rejected
		{name: "micro-units round up to a byte", memory: "500u", wantMemory: "1"},   // 1, unconverted: rejected

		// String form changes, byte count does not: Compose reads both spellings
		// as the same number.
		{name: "a fractional mebibyte picks a smaller unit", memory: "1.5Mi", wantMemory: "1536K"}, // 1572864, as 1.5M was
		{name: "leading zeros are normalised", memory: "000", wantMemory: "0"},                     // limits {} either way
		{name: "a trailing zero is normalised", cpu: "1.0", wantCPU: "1"},                          // cpus: 1

		// The b > 0 guard, in both directions. Zero must stay "0" rather than
		// becoming "0G", and a negative must fall through to a byte count rather
		// than being shifted into a unit. Compose rejects a negative limit either
		// way, so this pins the shape of the value, not a working file.
		{name: "zero stays a bare zero", memory: "0", wantMemory: "0"},
		{name: "negative falls through to bytes", memory: "-1Gi", wantMemory: "-1073741824"},

		// Both limits empty is not "two empty strings": addDeploy omits the
		// resources block altogether, so there is nothing for the converters to
		// be asked about.
		{name: "empty limits emit no resources block", cpu: "", memory: "", wantNoResources: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name: "limitsapp",
				Slices: []plan.Slice{
					{
						Name:      "api",
						Kind:      plan.SliceKindWeb,
						Image:     "api:latest",
						Port:      8080,
						Resources: &plan.ResourceSpec{CPULimit: tt.cpu, MemoryLimit: tt.memory},
					},
				},
			}

			result, err := local.New().Compile(context.Background(), p)
			require.NoError(t, err)

			var cf composeFile
			require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &cf))

			api := cf.Services["api"]
			require.NotNil(t, api.Deploy)

			if tt.wantNoResources {
				assert.Nil(t, api.Deploy.Resources)

				return
			}

			require.NotNil(t, api.Deploy.Resources)
			assert.Equal(t, tt.wantCPU, api.Deploy.Resources.Limits.CPUs)
			assert.Equal(t, tt.wantMemory, api.Deploy.Resources.Limits.Memory)
		})
	}
}
