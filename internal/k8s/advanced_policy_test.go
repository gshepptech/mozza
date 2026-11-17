package k8s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
)

func TestCompiler_NetworkPolicy(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "nptest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				NetworkPolicy: &plan.NetworkPolicySpec{
					AllowFrom: []string{"frontend", "worker"},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var npDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "NetworkPolicy" {
			npDoc = doc
			break
		}
	}
	require.NotNil(t, npDoc, "NetworkPolicy should be generated")
	assert.Equal(t, "networking.k8s.io/v1", npDoc["apiVersion"])

	spec := npDoc["spec"].(map[string]any)
	ingress := spec["ingress"].([]any)
	require.Len(t, ingress, 2)
}

func TestCompiler_NetworkPolicyDenyAll(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "denytest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "db",
				Kind:     plan.SliceKindDatabase,
				Image:    "postgres:16",
				Port:     5432,
				Replicas: 1,
				NetworkPolicy: &plan.NetworkPolicySpec{
					DenyAll: true,
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var npDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "NetworkPolicy" {
			npDoc = doc
			break
		}
	}
	require.NotNil(t, npDoc)

	spec := npDoc["spec"].(map[string]any)
	// Empty ingress rules means deny-all; serializes as nil or empty.
	ingress, ok := spec["ingress"]
	if ok && ingress != nil {
		assert.Len(t, ingress.([]any), 0, "deny-all should have empty ingress rules")
	}
	// If ingress key is absent or nil, that also means deny-all, so pass.
}

// --- HPA, PDB, SecurityContext, scheduling, update strategy ---

func TestCompiler_HPA(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "hpatest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 2,
				AutoScale: &plan.AutoScaleSpec{
					MinReplicas:  2,
					MaxReplicas:  10,
					CPUTarget:    80,
					MemoryTarget: 70,
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var hpaDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "HorizontalPodAutoscaler" {
			hpaDoc = doc
			break
		}
	}
	require.NotNil(t, hpaDoc, "HPA should be generated")
	assert.Equal(t, "autoscaling/v2", hpaDoc["apiVersion"])

	spec := hpaDoc["spec"].(map[string]any)
	assert.Equal(t, 2, spec["minReplicas"])
	assert.Equal(t, 10, spec["maxReplicas"])

	scaleRef := spec["scaleTargetRef"].(map[string]any)
	assert.Equal(t, "Deployment", scaleRef["kind"])
	assert.Equal(t, "api", scaleRef["name"])

	metrics := spec["metrics"].([]any)
	require.Len(t, metrics, 2)
}

func TestCompiler_HPAStatefulSet(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "hpasstest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "db",
				Kind:     plan.SliceKindStateful,
				Image:    "myorg/db:1.0",
				Port:     5432,
				Replicas: 3,
				AutoScale: &plan.AutoScaleSpec{
					MinReplicas: 3,
					MaxReplicas: 6,
					CPUTarget:   70,
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var hpaDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "HorizontalPodAutoscaler" {
			hpaDoc = doc
			break
		}
	}
	require.NotNil(t, hpaDoc)

	spec := hpaDoc["spec"].(map[string]any)
	scaleRef := spec["scaleTargetRef"].(map[string]any)
	assert.Equal(t, "StatefulSet", scaleRef["kind"])
}

func TestCompiler_PDB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		budget  *plan.DisruptionBudgetSpec
		checkFn func(t *testing.T, spec map[string]any)
	}{
		{
			name:   "min available",
			budget: &plan.DisruptionBudgetSpec{MinAvailable: 2},
			checkFn: func(t *testing.T, spec map[string]any) {
				assert.Equal(t, 2, spec["minAvailable"])
			},
		},
		{
			name:   "max unavailable",
			budget: &plan.DisruptionBudgetSpec{MaxUnavailable: 1},
			checkFn: func(t *testing.T, spec map[string]any) {
				assert.Equal(t, 1, spec["maxUnavailable"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name:      "pdbtest",
				Namespace: "prod",
				Slices: []plan.Slice{
					{
						Name:             "api",
						Kind:             plan.SliceKindWeb,
						Image:            "myorg/api:1.0",
						Port:             8080,
						Replicas:         3,
						DisruptionBudget: tt.budget,
					},
				},
			}

			c := k8s.New()
			result, err := c.Compile(context.Background(), p)
			require.NoError(t, err)

			var pdbDoc map[string]any
			for _, f := range result.Files {
				var doc map[string]any
				require.NoError(t, yaml.Unmarshal(f.Content, &doc))
				if doc["kind"] == "PodDisruptionBudget" {
					pdbDoc = doc
					break
				}
			}
			require.NotNil(t, pdbDoc, "PDB should be generated")
			assert.Equal(t, "policy/v1", pdbDoc["apiVersion"])

			spec := pdbDoc["spec"].(map[string]any)
			tt.checkFn(t, spec)
		})
	}
}

func TestCompiler_SecurityContext(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "sectest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Security: &plan.SecuritySpec{
					RunAsUser:        1000,
					RunAsGroup:       1000,
					ReadOnlyRoot:     true,
					DropCapabilities: []string{"ALL"},
					AddCapabilities:  []string{"NET_BIND_SERVICE"},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &doc))

	spec := doc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)

	// Pod-level security context.
	podSC := podSpec["securityContext"].(map[string]any)
	assert.Equal(t, 1000, podSC["runAsUser"])
	assert.Equal(t, 1000, podSC["runAsGroup"])

	// Container-level security context.
	containers := podSpec["containers"].([]any)
	container := containers[0].(map[string]any)
	csc := container["securityContext"].(map[string]any)
	assert.Equal(t, true, csc["readOnlyRootFilesystem"])
	assert.Equal(t, false, csc["allowPrivilegeEscalation"])

	caps := csc["capabilities"].(map[string]any)
	drop := caps["drop"].([]any)
	assert.Contains(t, drop, "ALL")
	add := caps["add"].([]any)
	assert.Contains(t, add, "NET_BIND_SERVICE")
}
