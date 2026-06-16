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

// --- Item 9: Init containers, sidecars, lifecycle hooks, graceful shutdown ---

func TestCompiler_InitContainers(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "inittest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				InitSteps: []plan.InitStep{
					{
						Image:   "busybox:latest",
						Command: "sh -c echo hello",
					},
					{
						Image: "myorg/migrate:1.0",
						Env:   map[string]string{"DB_URL": "postgres://db/app"},
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

	initContainers := podSpec["initContainers"].([]any)
	require.Len(t, initContainers, 2)

	init0 := initContainers[0].(map[string]any)
	assert.Equal(t, "init-0", init0["name"])
	assert.Equal(t, "busybox:latest", init0["image"])
	cmd := init0["command"].([]any)
	require.Len(t, cmd, 4)
	assert.Equal(t, "sh", cmd[0])
	assert.Equal(t, "-c", cmd[1])
	assert.Equal(t, "echo", cmd[2])
	assert.Equal(t, "hello", cmd[3])

	init1 := initContainers[1].(map[string]any)
	assert.Equal(t, "init-1", init1["name"])
	assert.Equal(t, "myorg/migrate:1.0", init1["image"])
	envList := init1["env"].([]any)
	require.Len(t, envList, 1)
	assert.Equal(t, "DB_URL", envList[0].(map[string]any)["name"])
}

func TestCompiler_Sidecars(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "sidecartest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Sidecars: []plan.Sidecar{
					{
						Name:  "envoy",
						Image: "envoyproxy/envoy:v1.28",
						Ports: []plan.PortSpec{{Name: "proxy", Port: 9901}},
						Env:   map[string]string{"LOG_LEVEL": "debug"},
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
	containers := podSpec["containers"].([]any)

	require.Len(t, containers, 2, "should have main + sidecar")

	sidecar := containers[1].(map[string]any)
	assert.Equal(t, "envoy", sidecar["name"])
	assert.Equal(t, "envoyproxy/envoy:v1.28", sidecar["image"])
	ports := sidecar["ports"].([]any)
	require.Len(t, ports, 1)
	assert.Equal(t, 9901, ports[0].(map[string]any)["containerPort"])
}

func TestCompiler_LifecycleHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lifecycle *plan.LifecycleSpec
		checkFn   func(t *testing.T, container map[string]any)
	}{
		{
			name: "pre-stop command",
			lifecycle: &plan.LifecycleSpec{
				PreStopCommand: "/bin/sh -c nginx -s quit",
			},
			checkFn: func(t *testing.T, container map[string]any) {
				lc := container["lifecycle"].(map[string]any)
				preStop := lc["preStop"].(map[string]any)
				exec := preStop["exec"].(map[string]any)
				cmd := exec["command"].([]any)
				assert.Equal(t, "/bin/sh", cmd[0])
			},
		},
		{
			name: "pre-stop wait",
			lifecycle: &plan.LifecycleSpec{
				PreStopWait: 15,
			},
			checkFn: func(t *testing.T, container map[string]any) {
				lc := container["lifecycle"].(map[string]any)
				preStop := lc["preStop"].(map[string]any)
				exec := preStop["exec"].(map[string]any)
				cmd := exec["command"].([]any)
				assert.Equal(t, "sleep", cmd[0])
				assert.Equal(t, "15", cmd[1])
			},
		},
		{
			name: "post-start command",
			lifecycle: &plan.LifecycleSpec{
				PostStartCommand: "/bin/sh -c warmup",
			},
			checkFn: func(t *testing.T, container map[string]any) {
				lc := container["lifecycle"].(map[string]any)
				postStart := lc["postStart"].(map[string]any)
				exec := postStart["exec"].(map[string]any)
				cmd := exec["command"].([]any)
				assert.Equal(t, "/bin/sh", cmd[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{
				Name: "lctest",
				Slices: []plan.Slice{{
					Name:      "api",
					Kind:      plan.SliceKindWeb,
					Image:     "myorg/api:1.0",
					Port:      8080,
					Replicas:  1,
					Lifecycle: tt.lifecycle,
				}},
			}

			c := k8s.New()
			result, err := c.Compile(context.Background(), p)
			require.NoError(t, err)

			var doc map[string]any
			require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &doc))
			spec := doc["spec"].(map[string]any)
			template := spec["template"].(map[string]any)
			podSpec := template["spec"].(map[string]any)
			containers := podSpec["containers"].([]any)
			container := containers[0].(map[string]any)

			tt.checkFn(t, container)
		})
	}
}

func TestCompiler_GracefulShutdown(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "gracetest",
		Slices: []plan.Slice{
			{
				Name:             "api",
				Kind:             plan.SliceKindWeb,
				Image:            "myorg/api:1.0",
				Port:             8080,
				Replicas:         1,
				GracefulShutdown: 60,
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

	assert.Equal(t, 60, podSpec["terminationGracePeriodSeconds"])
}

func TestCompiler_InitContainersOnStatefulSet(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "initsstest",
		Slices: []plan.Slice{
			{
				Name:     "db",
				Kind:     plan.SliceKindDatabase,
				Image:    "postgres:16",
				Port:     5432,
				Replicas: 1,
				InitSteps: []plan.InitStep{
					{Image: "busybox:latest", Command: "chmod 700 /var/lib/data"},
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

	initContainers := podSpec["initContainers"].([]any)
	require.Len(t, initContainers, 1)
	assert.Equal(t, "init-0", initContainers[0].(map[string]any)["name"])
}

func TestCompiler_InitContainersOnJob(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "initjobtest",
		Slices: []plan.Slice{
			{
				Name:  "migrate",
				Kind:  plan.SliceKindTask,
				Image: "myorg/migrate:1.0",
				InitSteps: []plan.InitStep{
					{Image: "busybox:latest", Command: "echo preparing"},
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

	initContainers := podSpec["initContainers"].([]any)
	require.Len(t, initContainers, 1)
}

// --- Item 10: RBAC, ConfigMap, Secret volumes, NetworkPolicy ---

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

func TestCompiler_ConfigMapFromMounts(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "cmtest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Mounts: []plan.MountSpec{
					{
						Type:   "configmap",
						Source: "app-config",
						Target: "/etc/config",
					},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var cmDoc map[string]any
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		if doc["kind"] == "ConfigMap" {
			cmDoc = doc
			break
		}
	}
	require.NotNil(t, cmDoc, "ConfigMap should be generated")
	assert.Equal(t, "app-config", cmDoc["metadata"].(map[string]any)["name"])
}

func TestCompiler_MountsVolumes(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "mounttest2",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Mounts: []plan.MountSpec{
					{Type: "pvc", Source: "data-vol", Target: "/data"},
					{Type: "secret", Source: "tls-certs", Target: "/etc/tls", ReadOnly: true},
					{Type: "emptydir", Source: "tmp", Target: "/tmp"},
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

	volumes := podSpec["volumes"].([]any)
	require.Len(t, volumes, 3)

	// PVC volume.
	pvcVol := volumes[0].(map[string]any)
	assert.Equal(t, "mount-data-vol", pvcVol["name"])
	assert.NotNil(t, pvcVol["persistentVolumeClaim"])

	// Secret volume.
	secretVol := volumes[1].(map[string]any)
	assert.Equal(t, "mount-tls-certs", secretVol["name"])
	assert.NotNil(t, secretVol["secret"])

	// EmptyDir volume.
	emptyVol := volumes[2].(map[string]any)
	assert.Equal(t, "mount-tmp", emptyVol["name"])
	assert.NotNil(t, emptyVol["emptyDir"])

	// Volume mounts.
	containers := podSpec["containers"].([]any)
	container := containers[0].(map[string]any)
	mounts := container["volumeMounts"].([]any)
	require.Len(t, mounts, 3)

	assert.Equal(t, "/data", mounts[0].(map[string]any)["mountPath"])
	assert.Equal(t, "/etc/tls", mounts[1].(map[string]any)["mountPath"])
	assert.Equal(t, true, mounts[1].(map[string]any)["readOnly"])
	assert.Equal(t, "/tmp", mounts[2].(map[string]any)["mountPath"])
}

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
	// If ingress key is absent or nil, that also means deny-all — pass.
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

// --- Item 11: HPA, PDB, SecurityContext, scheduling, update strategy ---

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

func TestCompiler_ResourceOrdering(t *testing.T) {
	t.Parallel()

	// Verify the overall resource emission order with advanced features.
	p := &plan.AppPlan{
		Name:      "ordertest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 2,
				Public:   true,
				Permissions: []plan.Permission{
					{Verbs: []string{"get"}, Resources: []string{"pods"}},
				},
				NetworkPolicy: &plan.NetworkPolicySpec{
					AllowFrom: []string{"frontend"},
				},
				AutoScale: &plan.AutoScaleSpec{
					MinReplicas: 2, MaxReplicas: 5, CPUTarget: 80,
				},
				DisruptionBudget: &plan.DisruptionBudgetSpec{
					MinAvailable: 1,
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	var kinds []string
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc))
		kinds = append(kinds, doc["kind"].(string))
	}

	// Expected order: SA → Role → RoleBinding → Deployment → Service → NetworkPolicy → HPA → PDB → Ingress
	expected := []string{
		"ServiceAccount", "Role", "RoleBinding",
		"Deployment", "Service", "NetworkPolicy",
		"HorizontalPodAutoscaler", "PodDisruptionBudget", "Ingress",
	}
	assert.Equal(t, expected, kinds)
}

func TestCompiler_AllYAMLValid(t *testing.T) {
	t.Parallel()

	// Comprehensive test with all advanced features to verify YAML validity.
	p := &plan.AppPlan{
		Name:      "fulltest",
		Namespace: "prod",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 2,
				InitSteps: []plan.InitStep{
					{Image: "busybox:latest", Command: "echo init"},
				},
				Sidecars: []plan.Sidecar{
					{Name: "proxy", Image: "envoy:latest"},
				},
				Lifecycle: &plan.LifecycleSpec{
					PreStopCommand: "kill -SIGTERM 1",
				},
				GracefulShutdown: 30,
				Permissions: []plan.Permission{
					{Verbs: []string{"get"}, Resources: []string{"pods"}},
				},
				Mounts: []plan.MountSpec{
					{Type: "configmap", Source: "cfg", Target: "/etc/cfg"},
				},
				Security: &plan.SecuritySpec{
					RunAsUser:        1000,
					ReadOnlyRoot:     true,
					DropCapabilities: []string{"ALL"},
				},
				Scheduling: &plan.SchedulingSpec{
					AntiAffinity:   true,
					SpreadTopology: "topology.kubernetes.io/zone",
				},
				AutoScale: &plan.AutoScaleSpec{
					MinReplicas: 2, MaxReplicas: 10, CPUTarget: 80,
				},
				DisruptionBudget: &plan.DisruptionBudgetSpec{MinAvailable: 1},
				UpdateStrategy:   &plan.UpdateStrategySpec{MaxSurge: "1", MaxUnavailable: "0"},
				NetworkPolicy:    &plan.NetworkPolicySpec{AllowFrom: []string{"frontend"}},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	for _, f := range result.Files {
		var doc map[string]any
		err := yaml.Unmarshal(f.Content, &doc)
		require.NoError(t, err, "invalid YAML in %s", f.Path)
		assert.NotEmpty(t, doc, "empty document in %s", f.Path)
	}
}
