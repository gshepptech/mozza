package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

// postDetectGenerate sends a generate request and decodes the response. The body
// is decoded as strings throughout because every key the endpoint can return
// (recipe, dockerfile, error) holds one.
func postDetectGenerate(
	t *testing.T,
	ts *httptest.Server,
	cookie *http.Cookie,
	payload map[string]any,
) (int, map[string]string) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/detect/generate", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var decoded map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))

	return resp.StatusCode, decoded
}

// buildPlanFromRecipe runs recipe source through the exact chain
// Server.planFromSource runs, so a recipe that survives this is one
// POST /api/v1/compile would accept rather than merely one that lexes.
func buildPlanFromRecipe(t *testing.T, source string) *plan.AppPlan {
	t.Helper()
	rec, err := recipe.NewParser(source).Parse()
	require.NoError(t, err, "recipe must parse:\n%s", source)

	p, err := plan.Build(rec)
	require.NoError(t, err, "plan must build:\n%s", source)
	require.NoError(t, plan.Validate(p), "plan must validate:\n%s", source)

	return p
}

// sliceNames lists a plan's slice names in emission order.
func sliceNames(p *plan.AppPlan) []string {
	names := make([]string, len(p.Slices))
	for i, s := range p.Slices {
		names[i] = s.Name
	}
	return names
}

func TestDetectGenerate_EveryBranchYieldsAUsableRecipe(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "detect-branches@example.com", "Test", "password123")

	// storage is asserted on the database rows: a size emitted in a form the
	// parser does not accept fails the whole recipe rather than going missing
	// quietly.
	tests := []struct {
		name    string
		choices map[string]string
		slices  []string
		storage string
	}{
		{
			name:   "no choices",
			slices: []string{"web"},
		},
		{
			name:    "postgres",
			choices: map[string]string{"database": "postgres"},
			slices:  []string{"web", "database"},
			storage: "10Gi",
		},
		{
			name:    "mysql",
			choices: map[string]string{"database": "mysql"},
			slices:  []string{"web", "database"},
			storage: "10Gi",
		},
		{
			name:    "mongodb",
			choices: map[string]string{"database": "mongodb"},
			slices:  []string{"web", "database"},
			storage: "10Gi",
		},
		{
			name:    "worker with no name falls back to the default",
			choices: map[string]string{"worker": "yes"},
			slices:  []string{"web", "worker"},
		},
		{
			name:    "named worker",
			choices: map[string]string{"worker": "yes", "worker_name": "mailer"},
			slices:  []string{"web", "mailer"},
		},
		{
			name:    "cache",
			choices: map[string]string{"cache": "yes"},
			slices:  []string{"web", "cache"},
		},
		{
			name: "every branch at once",
			choices: map[string]string{
				"database":    "postgres",
				"worker":      "yes",
				"worker_name": "mailer",
				"cache":       "yes",
			},
			slices:  []string{"web", "database", "mailer", "cache"},
			storage: "10Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := postDetectGenerate(t, ts, cookie, map[string]any{
				"framework":    "nextjs",
				"language":     "javascript",
				"app_name":     "shop",
				"port":         3000,
				"user_choices": tt.choices,
			})
			require.Equal(t, http.StatusOK, status, "error: %s", body["error"])
			require.NotEmpty(t, body["recipe"])

			p := buildPlanFromRecipe(t, body["recipe"])
			assert.Equal(t, tt.slices, sliceNames(p))
			require.NotNil(t, p.Slices[0].Resources, "both limit lines must survive")
			assert.Equal(t, "500m", p.Slices[0].Resources.CPULimit)
			assert.Equal(t, "256Mi", p.Slices[0].Resources.MemoryLimit)

			if tt.storage != "" {
				assert.Equal(t, plan.SliceKindStateful, p.Slices[1].Kind)
				assert.Equal(t, tt.storage, p.Slices[1].StatefulStorage)
			}
		})
	}
}

func TestDetectGenerate_RejectsValuesThatCarryRecipeStructure(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "detect-inject@example.com", "Test", "password123")

	tests := []struct {
		name    string
		payload map[string]any
		field   string
	}{
		{
			// The emitter appends its own colon, so a payload that carries one
			// closes the header cleanly. Unguarded this yields a valid 3-slice
			// recipe compiling to a w-deployment.yaml running evil:latest at 9
			// replicas plus an attacker-named backdoor-deployment.yaml.
			name: "worker_name closing its own header",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "shop",
				"user_choices": map[string]string{
					"worker":      "yes",
					"worker_name": "w:\n  from image evil:latest\n  run 9 copies\nBackdoor",
				},
			},
			field: "worker_name",
		},
		{
			// Without the colon the emitter's own one lands on the payload's
			// last line, so this corrupts the recipe rather than injecting into
			// it. Refused all the same.
			name: "worker_name spanning lines without a colon",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "shop",
				"user_choices": map[string]string{
					"worker":      "yes",
					"worker_name": "w\n  run 5 copies\n  from image evil:latest\nBackdoor",
				},
			},
			field: "worker_name",
		},
		{
			// worker_name is checked whenever it is present. Today it only
			// reaches the recipe when worker is "yes", so this 400s on a field
			// that would not have been written. That is deliberate, so the guard
			// cannot be bypassed by omitting the flag.
			name: "worker_name sent with no worker key",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "shop",
				"user_choices": map[string]string{
					"worker_name": "w:\n  from image evil:latest\nBackdoor",
				},
			},
			field: "worker_name",
		},
		{
			// A valid DNS label the lexer dispatches specially instead of
			// reading as a slice header.
			name: "worker_name naming a reserved header",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "shop",
				"user_choices": map[string]string{
					"worker":      "yes",
					"worker_name": "app",
				},
			},
			field: "worker_name",
		},
		{
			name: "worker_name duplicating the emitter's own header",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "shop",
				"user_choices": map[string]string{
					"worker":      "yes",
					"worker_name": "web",
				},
			},
			field: "worker_name",
		},
		{
			// The trailing # re-opens a comment to absorb the emitter's " (%s)"
			// tail, so unguarded this yields a valid 2-slice recipe. The
			// injected directives each begin their own line, so the grammar's
			// begins-a-line rule does not close it.
			name: "framework re-opening its comment",
			payload: map[string]any{
				"framework": "nextjs\nBackdoor:\n  from image evil:latest\n  run 9 copies\n#",
				"app_name":  "shop",
			},
			field: "framework",
		},
		{
			name: "language re-opening its comment",
			payload: map[string]any{
				"framework": "nextjs",
				"language":  "javascript\nBackdoor:\n  from image evil:latest\n  run 9 copies\n#",
				"app_name":  "shop",
			},
			field: "language",
		},
		{
			name: "app_name spanning lines",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "shop\nBackdoor:\n  from image evil:latest",
			},
			field: "app_name",
		},
		{
			// The wizard's ordinary path: two bare inputs with no pattern, so a
			// user typing a display name lands here.
			name: "app_name holding a space",
			payload: map[string]any{
				"framework": "nextjs",
				"app_name":  "My Shop",
			},
			field: "app_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := postDetectGenerate(t, ts, cookie, tt.payload)
			require.Equal(t, http.StatusBadRequest, status,
				"expected 400, got recipe:\n%s", body["recipe"])
			assert.Contains(t, body["error"], tt.field,
				"the error must name the field the caller has to fix")
			assert.Empty(t, body["recipe"], "a refused request emits no recipe")
		})
	}
}
