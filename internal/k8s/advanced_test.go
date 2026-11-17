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

// --- Init containers, sidecars, lifecycle hooks, graceful shutdown ---

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
