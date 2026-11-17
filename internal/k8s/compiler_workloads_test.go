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

func TestCompiler_Compile_DeploymentReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		replicas     int
		wantReplicas int
	}{
		{name: "explicit replicas", replicas: 3, wantReplicas: 3},
		{name: "zero replicas defaults to one", replicas: 0, wantReplicas: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name: "reptest",
				Slices: []plan.Slice{
					{
						Name:     "api",
						Kind:     plan.SliceKindWeb,
						Image:    "myorg/api:1.0",
						Port:     8080,
						Replicas: tt.replicas,
					},
				},
			}

			c := k8s.New()
			result, err := c.Compile(context.Background(), p)
			require.NoError(t, err)

			var doc map[string]any
			require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &doc))

			spec, ok := doc["spec"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.wantReplicas, spec["replicas"])
		})
	}
}

func TestCompiler_Compile_PVCStorage(t *testing.T) {
	t.Parallel()

	// Use a cache slice for PVC test since database slices now produce StatefulSets.
	p := &plan.AppPlan{
		Name: "pvctest",
		Slices: []plan.Slice{
			{
				Name:     "redis",
				Kind:     plan.SliceKindCache,
				Image:    "redis:7",
				Port:     6379,
				Replicas: 1,
				Cache:    &plan.CacheSpec{Storage: "50Gi"},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// PVC is the first file (pvc, deployment, service).
	require.Len(t, result.Files, 3)
	pvcFile := result.Files[0]
	assert.Equal(t, "k8s/redis-persistentvolumeclaim.yaml", pvcFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(pvcFile.Content, &doc))
	assert.Equal(t, "v1", doc["apiVersion"])
	assert.Equal(t, "PersistentVolumeClaim", doc["kind"])

	spec := doc["spec"].(map[string]any)
	accessModes := spec["accessModes"].([]any)
	assert.Equal(t, "ReadWriteOnce", accessModes[0])

	resources := spec["resources"].(map[string]any)
	requests := resources["requests"].(map[string]any)
	assert.Equal(t, "50Gi", requests["storage"])
}

func TestCompiler_Compile_CacheWithoutStorage(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "nostorage",
		Slices: []plan.Slice{
			{
				Name:     "redis",
				Kind:     plan.SliceKindCache,
				Image:    "redis:7",
				Port:     6379,
				Replicas: 1,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// No PVC without storage spec: deployment + service = 2 files.
	assert.Len(t, result.Files, 2)
	paths := make([]string, len(result.Files))
	for i, f := range result.Files {
		paths[i] = f.Path
	}
	assert.NotContains(t, paths, "k8s/redis-pvc.yaml")
}

func TestCompiler_Compile_DeploymentVolumeMounts(t *testing.T) {
	t.Parallel()

	// Use a cache slice for volume mount test since it still generates Deployments.
	p := &plan.AppPlan{
		Name: "voltest",
		Slices: []plan.Slice{
			{
				Name:     "redis",
				Kind:     plan.SliceKindCache,
				Image:    "redis:7",
				Port:     6379,
				Replicas: 1,
				Cache:    &plan.CacheSpec{Storage: "10Gi"},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Deployment is at index 1 (after PVC at 0).
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[1].Content, &doc))

	spec := doc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)

	// Check volumes.
	volumes := podSpec["volumes"].([]any)
	require.Len(t, volumes, 1)
	vol := volumes[0].(map[string]any)
	assert.Equal(t, "redis-data", vol["name"])
	pvcRef := vol["persistentVolumeClaim"].(map[string]any)
	assert.Equal(t, "redis", pvcRef["claimName"])

	// Check volume mounts.
	containers := podSpec["containers"].([]any)
	container := containers[0].(map[string]any)
	mounts := container["volumeMounts"].([]any)
	require.Len(t, mounts, 1)
	mount := mounts[0].(map[string]any)
	assert.Equal(t, "redis-data", mount["name"])
	assert.Equal(t, "/var/lib/data", mount["mountPath"])
}

func TestCompiler_Compile_EnvVars(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "envtest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Env: map[string]string{
					"DATABASE_URL": "postgres://localhost/db",
					"LOG_LEVEL":    "info",
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
	container := containers[0].(map[string]any)

	envList, ok := container["env"].([]any)
	require.True(t, ok, "env should be present in container spec")
	require.Len(t, envList, 2)

	// Env vars should be sorted by key name.
	env0 := envList[0].(map[string]any)
	assert.Equal(t, "DATABASE_URL", env0["name"])
	assert.Equal(t, "postgres://localhost/db", env0["value"])

	env1 := envList[1].(map[string]any)
	assert.Equal(t, "LOG_LEVEL", env1["name"])
	assert.Equal(t, "info", env1["value"])
}

func TestCompiler_Compile_ResourceLimits(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "restest",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Resources: &plan.ResourceSpec{
					CPULimit:    "500m",
					MemoryLimit: "256Mi",
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
	container := containers[0].(map[string]any)

	resources, ok := container["resources"].(map[string]any)
	require.True(t, ok, "resources should be present in container spec")

	limits, ok := resources["limits"].(map[string]any)
	require.True(t, ok, "limits should be present in resources")
	assert.Equal(t, "500m", limits["cpu"])
	assert.Equal(t, "256Mi", limits["memory"])
}

func TestCompiler_Compile_MountPathFromSpec(t *testing.T) {
	t.Parallel()

	// Use a cache slice with custom mount path since database slices now produce StatefulSets.
	p := &plan.AppPlan{
		Name: "mounttest",
		Slices: []plan.Slice{
			{
				Name:     "redis",
				Kind:     plan.SliceKindCache,
				Image:    "redis:7",
				Port:     6379,
				Replicas: 1,
				Cache: &plan.CacheSpec{
					Storage:   "10Gi",
					MountPath: "/data/redis",
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Deployment is at index 1 (after PVC at 0).
	require.True(t, len(result.Files) >= 2)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[1].Content, &doc))

	spec := doc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	container := containers[0].(map[string]any)

	mounts := container["volumeMounts"].([]any)
	require.Len(t, mounts, 1)
	mount := mounts[0].(map[string]any)
	assert.Equal(t, "/data/redis", mount["mountPath"], "mount path should use CacheSpec.MountPath")
}

func TestCompiler_Compile_StatefulSetDetails(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "sstest",
		Slices: []plan.Slice{
			{
				Name:            "db",
				Kind:            plan.SliceKindDatabase,
				Image:           "postgres:16",
				Port:            5432,
				Replicas:        3,
				OrderedStartup:  true,
				StatefulStorage: "20Gi",
				Database:        &plan.DatabaseSpec{Storage: "20Gi"},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// StatefulSet is the first file, headless service is the second.
	require.Len(t, result.Files, 2)
	ssFile := result.Files[0]
	assert.Equal(t, "k8s/db-statefulset.yaml", ssFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(ssFile.Content, &doc))
	assert.Equal(t, "apps/v1", doc["apiVersion"])
	assert.Equal(t, "StatefulSet", doc["kind"])

	spec := doc["spec"].(map[string]any)
	assert.Equal(t, "db", spec["serviceName"])
	assert.Equal(t, 3, spec["replicas"])
	assert.Equal(t, "OrderedReady", spec["podManagementPolicy"])

	// volumeClaimTemplates.
	vcts := spec["volumeClaimTemplates"].([]any)
	require.Len(t, vcts, 1)
	vct := vcts[0].(map[string]any)
	vctMeta := vct["metadata"].(map[string]any)
	assert.Equal(t, "data", vctMeta["name"])
}

func TestCompiler_Compile_JobDetails(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "jobtest",
		Slices: []plan.Slice{
			{
				Name:        "migrate",
				Kind:        plan.SliceKindTask,
				Image:       "myorg/migrate:1.0",
				RunOnce:     true,
				Retries:     3,
				Parallelism: 2,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	require.Len(t, result.Files, 1)
	jobFile := result.Files[0]
	assert.Equal(t, "k8s/migrate-job.yaml", jobFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(jobFile.Content, &doc))
	assert.Equal(t, "batch/v1", doc["apiVersion"])
	assert.Equal(t, "Job", doc["kind"])

	spec := doc["spec"].(map[string]any)
	assert.Equal(t, 3, spec["backoffLimit"])
	assert.Equal(t, 2, spec["parallelism"])
	assert.Equal(t, 2, spec["completions"])

	// RestartPolicy should be Never.
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	assert.Equal(t, "Never", podSpec["restartPolicy"])
}

func TestCompiler_Compile_CronJobDetails(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "crontest",
		Slices: []plan.Slice{
			{
				Name:     "backup",
				Kind:     plan.SliceKindScheduled,
				Image:    "myorg/backup:1.0",
				Schedule: "0 2 * * *",
				Retries:  2,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	require.Len(t, result.Files, 1)
	cronFile := result.Files[0]
	assert.Equal(t, "k8s/backup-cronjob.yaml", cronFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(cronFile.Content, &doc))
	assert.Equal(t, "batch/v1", doc["apiVersion"])
	assert.Equal(t, "CronJob", doc["kind"])

	spec := doc["spec"].(map[string]any)
	assert.Equal(t, "0 2 * * *", spec["schedule"])
	assert.Equal(t, "Forbid", spec["concurrencyPolicy"])
	assert.Equal(t, 3, spec["successfulJobsHistoryLimit"])
	assert.Equal(t, 1, spec["failedJobsHistoryLimit"])
}

func TestCompiler_Compile_DaemonSetDetails(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "dstest",
		Slices: []plan.Slice{
			{
				Name:  "log-agent",
				Kind:  plan.SliceKindDaemon,
				Image: "myorg/log-agent:1.0",
				Port:  9100,
				Scheduling: &plan.SchedulingSpec{
					NodeRequirements: []plan.LabelConstraint{
						{Key: "node-role", Value: "worker"},
					},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// DaemonSet + Service (has port).
	require.Len(t, result.Files, 2)
	dsFile := result.Files[0]
	assert.Equal(t, "k8s/log-agent-daemonset.yaml", dsFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(dsFile.Content, &doc))
	assert.Equal(t, "apps/v1", doc["apiVersion"])
	assert.Equal(t, "DaemonSet", doc["kind"])

	spec := doc["spec"].(map[string]any)
	updateStrategy := spec["updateStrategy"].(map[string]any)
	assert.Equal(t, "RollingUpdate", updateStrategy["type"])

	// Check nodeSelector.
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	nodeSelector := podSpec["nodeSelector"].(map[string]any)
	assert.Equal(t, "worker", nodeSelector["node-role"])

	// No replicas field.
	_, hasReplicas := spec["replicas"]
	assert.False(t, hasReplicas, "DaemonSet should not have replicas field")
}
