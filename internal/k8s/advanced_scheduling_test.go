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

func TestCompiler_NodeAffinityRequired(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "affinitytest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Scheduling: &plan.SchedulingSpec{
					NodeRequirements: []plan.LabelConstraint{
						{Key: "disktype", Value: "ssd"},
					},
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
	affinity := podSpec["affinity"].(map[string]any)
	nodeAffinity := affinity["nodeAffinity"].(map[string]any)

	required := nodeAffinity["requiredDuringSchedulingIgnoredDuringExecution"].(map[string]any)
	terms := required["nodeSelectorTerms"].([]any)
	require.Len(t, terms, 1)
}

func TestCompiler_NodeAffinityPreferred(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "preftest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Scheduling: &plan.SchedulingSpec{
					NodePreferences: []plan.LabelConstraint{
						{Key: "zone", Value: "us-west-2a"},
					},
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
	affinity := podSpec["affinity"].(map[string]any)
	nodeAffinity := affinity["nodeAffinity"].(map[string]any)

	preferred := nodeAffinity["preferredDuringSchedulingIgnoredDuringExecution"].([]any)
	require.Len(t, preferred, 1)
	assert.Equal(t, 100, preferred[0].(map[string]any)["weight"])
}

func TestCompiler_PodAntiAffinity(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "antitest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 3,
				Scheduling: &plan.SchedulingSpec{
					AntiAffinity: true,
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
	affinity := podSpec["affinity"].(map[string]any)

	podAntiAffinity := affinity["podAntiAffinity"].(map[string]any)
	preferred := podAntiAffinity["preferredDuringSchedulingIgnoredDuringExecution"].([]any)
	require.Len(t, preferred, 1)

	term := preferred[0].(map[string]any)["podAffinityTerm"].(map[string]any)
	assert.Equal(t, "kubernetes.io/hostname", term["topologyKey"])
}

func TestCompiler_SpreadTopology(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "spreadtest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 3,
				Scheduling: &plan.SchedulingSpec{
					SpreadTopology: "topology.kubernetes.io/zone",
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

	tsc := podSpec["topologySpreadConstraints"].([]any)
	require.Len(t, tsc, 1)
	assert.Equal(t, "topology.kubernetes.io/zone", tsc[0].(map[string]any)["topologyKey"])
	assert.Equal(t, 1, tsc[0].(map[string]any)["maxSkew"])
}

func TestCompiler_DeploymentUpdateStrategy(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "updatetest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 3,
				UpdateStrategy: &plan.UpdateStrategySpec{
					MaxSurge:       "25%",
					MaxUnavailable: "0",
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
	strategy := spec["strategy"].(map[string]any)
	assert.Equal(t, "RollingUpdate", strategy["type"])
	rollingUpdate := strategy["rollingUpdate"].(map[string]any)
	assert.Equal(t, "25%", rollingUpdate["maxSurge"])
	assert.Equal(t, 0, rollingUpdate["maxUnavailable"])
}

func TestCompiler_StatefulSetUpdateStrategy(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "ssupdtest",
		Slices: []plan.Slice{
			{
				Name:     "db",
				Kind:     plan.SliceKindStateful,
				Image:    "myorg/db:1.0",
				Port:     5432,
				Replicas: 3,
				UpdateStrategy: &plan.UpdateStrategySpec{
					MaxUnavailable: "1",
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
	updateStrategy := spec["updateStrategy"].(map[string]any)
	assert.Equal(t, "RollingUpdate", updateStrategy["type"])
}
