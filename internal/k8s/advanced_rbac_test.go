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

// --- RBAC, ConfigMap, Secret volumes, NetworkPolicy ---

func TestCompiler_ServiceAccount(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "satest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Permissions: []plan.Permission{
					{
						Verbs:     []string{"get", "list"},
						Resources: []string{"pods"},
					},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Should have: SA, Role, RoleBinding, Deployment, Service.
	require.True(t, len(result.Files) >= 5)

	// First file should be ServiceAccount.
	var saDoc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &saDoc))
	assert.Equal(t, "ServiceAccount", saDoc["kind"])
	meta := saDoc["metadata"].(map[string]any)
	assert.Equal(t, "api", meta["name"])
	assert.Equal(t, "prod", meta["namespace"])
}

func TestCompiler_RBACRole(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "rbactest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Permissions: []plan.Permission{
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{"pods", "services"},
					},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Find the Role file.
	var roleDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "Role" {
			roleDoc = doc
			break
		}
	}
	require.NotNil(t, roleDoc, "Role should be generated")
	assert.Equal(t, "rbac.authorization.k8s.io/v1", roleDoc["apiVersion"])

	rules := roleDoc["rules"].([]any)
	require.Len(t, rules, 1)
	rule := rules[0].(map[string]any)
	verbs := rule["verbs"].([]any)
	assert.Len(t, verbs, 3)
}

func TestCompiler_ClusterRole(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "crtest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "operator",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/operator:1.0",
				Port:     8080,
				Replicas: 1,
				Permissions: []plan.Permission{
					{
						Verbs:       []string{"get", "list"},
						Resources:   []string{"nodes"},
						ClusterWide: true,
					},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var crDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "ClusterRole" {
			crDoc = doc
			break
		}
	}
	require.NotNil(t, crDoc, "ClusterRole should be generated")

	// ClusterRole should NOT have namespace.
	meta := crDoc["metadata"].(map[string]any)
	_, hasNS := meta["namespace"]
	assert.False(t, hasNS || meta["namespace"] == "", "ClusterRole should not have namespace")
}

func TestCompiler_RoleBinding(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "rbtest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Permissions: []plan.Permission{
					{
						Verbs:     []string{"get"},
						Resources: []string{"pods"},
					},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var rbDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "RoleBinding" {
			rbDoc = doc
			break
		}
	}
	require.NotNil(t, rbDoc, "RoleBinding should be generated")

	subjects := rbDoc["subjects"].([]any)
	require.Len(t, subjects, 1)
	assert.Equal(t, "ServiceAccount", subjects[0].(map[string]any)["kind"])
	assert.Equal(t, "api", subjects[0].(map[string]any)["name"])

	roleRef := rbDoc["roleRef"].(map[string]any)
	assert.Equal(t, "Role", roleRef["kind"])
	assert.Equal(t, "api", roleRef["name"])
}

func TestCompiler_ServiceAccountInPodSpec(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "sapodtest",
		Slices: []plan.Slice{
			{
				Name:           "api",
				Kind:           plan.SliceKindWeb,
				Image:          "myorg/api:1.0",
				Port:           8080,
				Replicas:       1,
				ServiceAccount: "custom-sa",
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Find the deployment.
	var depDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "Deployment" {
			depDoc = doc
			break
		}
	}
	require.NotNil(t, depDoc)

	spec := depDoc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	assert.Equal(t, "custom-sa", podSpec["serviceAccountName"])
}
