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

func TestCompiler_Name(t *testing.T) {
	t.Parallel()

	c := k8s.New()
	assert.Equal(t, "kubernetes", c.Name())
}

func TestCompiler_Compile_NilPlan(t *testing.T) {
	t.Parallel()

	c := k8s.New()
	_, err := c.Compile(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Compile:")
}

func TestCompiler_Compile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		plan       *plan.AppPlan
		wantFiles  int
		wantPaths  []string
		wantSummay string
	}{
		{
			name: "simple web service produces deployment and service",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:     "api",
						Kind:     plan.SliceKindWeb,
						Image:    "myorg/api:1.0",
						Port:     8080,
						Replicas: 2,
					},
				},
			},
			wantFiles: 2,
			wantPaths: []string{
				"k8s/api-deployment.yaml",
				"k8s/api-service.yaml",
			},
		},
		{
			name: "public web service produces deployment service and ingress",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:       "web",
						Kind:       plan.SliceKindWeb,
						Image:      "myorg/web:2.0",
						Port:       3000,
						Public:     true,
						Replicas:   1,
						HealthPath: "/healthz",
					},
				},
			},
			wantFiles: 3,
			wantPaths: []string{
				"k8s/web-deployment.yaml",
				"k8s/web-service.yaml",
				"k8s/web-ingress.yaml",
			},
		},
		{
			name: "database slice produces statefulset and headless service",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:     "db",
						Kind:     plan.SliceKindDatabase,
						Image:    "postgres:16",
						Port:     5432,
						Replicas: 1,
						Database: &plan.DatabaseSpec{Storage: "10Gi"},
					},
				},
			},
			wantFiles: 2,
			wantPaths: []string{
				"k8s/db-statefulset.yaml",
				"k8s/db-service.yaml",
			},
		},
		{
			name: "cache slice with storage produces pvc deployment and service",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:     "redis",
						Kind:     plan.SliceKindCache,
						Image:    "redis:7",
						Port:     6379,
						Replicas: 1,
						Cache:    &plan.CacheSpec{Storage: "1Gi"},
					},
				},
			},
			wantFiles: 3,
			wantPaths: []string{
				"k8s/redis-persistentvolumeclaim.yaml",
				"k8s/redis-deployment.yaml",
				"k8s/redis-service.yaml",
			},
		},
		{
			name: "task slice produces job only",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:    "migrate",
						Kind:    plan.SliceKindTask,
						Image:   "myorg/migrate:1.0",
						RunOnce: true,
						Retries: 3,
					},
				},
			},
			wantFiles: 1,
			wantPaths: []string{
				"k8s/migrate-job.yaml",
			},
		},
		{
			name: "scheduled slice produces cronjob only",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:     "backup",
						Kind:     plan.SliceKindScheduled,
						Image:    "myorg/backup:1.0",
						Schedule: "0 2 * * *",
					},
				},
			},
			wantFiles: 1,
			wantPaths: []string{
				"k8s/backup-cronjob.yaml",
			},
		},
		{
			name: "daemon slice produces daemonset only",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:  "log-agent",
						Kind:  plan.SliceKindDaemon,
						Image: "myorg/log-agent:1.0",
					},
				},
			},
			wantFiles: 1,
			wantPaths: []string{
				"k8s/log-agent-daemonset.yaml",
			},
		},
		{
			name: "stateful slice produces statefulset and headless service",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:            "zookeeper",
						Kind:            plan.SliceKindStateful,
						Image:           "zookeeper:3.8",
						Port:            2181,
						Replicas:        3,
						OrderedStartup:  true,
						StatefulStorage: "5Gi",
					},
				},
			},
			wantFiles: 2,
			wantPaths: []string{
				"k8s/zookeeper-statefulset.yaml",
				"k8s/zookeeper-service.yaml",
			},
		},
		{
			name: "worker slice without port produces deployment only",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:     "processor",
						Kind:     plan.SliceKindWorker,
						Image:    "myorg/processor:1.0",
						Port:     0,
						Replicas: 3,
					},
				},
			},
			wantFiles: 1,
			wantPaths: []string{
				"k8s/processor-deployment.yaml",
			},
		},
		{
			name: "full app with multiple slices",
			plan: &plan.AppPlan{
				Name: "fullstack",
				Slices: []plan.Slice{
					{
						Name:       "frontend",
						Kind:       plan.SliceKindWeb,
						Image:      "myorg/frontend:1.0",
						Port:       3000,
						Public:     true,
						Replicas:   2,
						HealthPath: "/health",
					},
					{
						Name:     "backend",
						Kind:     plan.SliceKindWeb,
						Image:    "myorg/backend:1.0",
						Port:     8080,
						Replicas: 2,
						Needs:    []string{"db", "cache"},
					},
					{
						Name:     "worker",
						Kind:     plan.SliceKindWorker,
						Image:    "myorg/worker:1.0",
						Replicas: 3,
					},
					{
						Name:     "db",
						Kind:     plan.SliceKindDatabase,
						Image:    "postgres:16",
						Port:     5432,
						Replicas: 1,
						Database: &plan.DatabaseSpec{Storage: "20Gi"},
					},
					{
						Name:     "cache",
						Kind:     plan.SliceKindCache,
						Image:    "redis:7",
						Port:     6379,
						Replicas: 1,
						Cache:    &plan.CacheSpec{Storage: "2Gi"},
					},
				},
			},
			// frontend: deployment + service + ingress = 3
			// backend:  deployment + service = 2
			// worker:   deployment = 1
			// db:       statefulset + headless service = 2 (no PVC — uses volumeClaimTemplates)
			// cache:    pvc + deployment + service = 3
			// total: 11
			wantFiles: 11,
			wantPaths: []string{
				// Global ordering: PVCs → Workloads → Services → Ingresses
				"k8s/cache-persistentvolumeclaim.yaml",
				"k8s/frontend-deployment.yaml",
				"k8s/backend-deployment.yaml",
				"k8s/worker-deployment.yaml",
				"k8s/db-statefulset.yaml",
				"k8s/cache-deployment.yaml",
				"k8s/frontend-service.yaml",
				"k8s/backend-service.yaml",
				"k8s/db-service.yaml",
				"k8s/cache-service.yaml",
				"k8s/frontend-ingress.yaml",
			},
		},
		{
			name:      "empty plan produces no files",
			plan:      &plan.AppPlan{Name: "empty"},
			wantFiles: 0,
			wantPaths: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := k8s.New()
			result, err := c.Compile(context.Background(), tt.plan)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Files, tt.wantFiles)
			assert.Contains(t, result.Summary, tt.plan.Name)

			var paths []string
			for _, f := range result.Files {
				paths = append(paths, f.Path)
			}
			assert.Equal(t, tt.wantPaths, paths)
		})
	}
}

func TestCompiler_Compile_YAMLValid(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "yamltest",
		Slices: []plan.Slice{
			{
				Name:       "api",
				Kind:       plan.SliceKindWeb,
				Image:      "myorg/api:1.0",
				Port:       8080,
				Public:     true,
				Replicas:   2,
				HealthPath: "/healthz",
			},
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

	for _, f := range result.Files {
		var doc map[string]any
		err := yaml.Unmarshal(f.Content, &doc)
		require.NoError(t, err, "invalid YAML in %s", f.Path)
		assert.NotEmpty(t, doc, "empty document in %s", f.Path)
	}
}

func TestCompiler_Compile_Labels(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "labelapp",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Check deployment labels.
	depFile := result.Files[0]
	assert.Equal(t, "k8s/api-deployment.yaml", depFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(depFile.Content, &doc))

	meta, ok := doc["metadata"].(map[string]any)
	require.True(t, ok)

	lbls, ok := meta["labels"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "labelapp", lbls["app.kubernetes.io/name"])
	assert.Equal(t, "api", lbls["app.kubernetes.io/component"])
	assert.Equal(t, "labelapp", lbls["app.kubernetes.io/part-of"])
	assert.Equal(t, "mozza", lbls["app.kubernetes.io/managed-by"])
}

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

func TestCompiler_Compile_ReadinessProbe(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "probetest",
		Slices: []plan.Slice{
			{
				Name:       "api",
				Kind:       plan.SliceKindWeb,
				Image:      "myorg/api:1.0",
				Port:       8080,
				Replicas:   1,
				HealthPath: "/ready",
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

	probe, ok := container["readinessProbe"].(map[string]any)
	require.True(t, ok, "readinessProbe should be present")

	httpGet, ok := probe["httpGet"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/ready", httpGet["path"])
	assert.Equal(t, 8080, httpGet["port"])
}

func TestCompiler_Compile_NoProbeWithoutHealthPath(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "noprobe",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
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

	_, ok := container["readinessProbe"]
	assert.False(t, ok, "readinessProbe should not be present without HealthPath")
}

func TestCompiler_Compile_IngressPath(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "ingtest",
		Slices: []plan.Slice{
			{
				Name:     "web",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/web:1.0",
				Port:     3000,
				Public:   true,
				Replicas: 1,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Ingress is the last file: deployment, service, ingress.
	require.Len(t, result.Files, 3)
	ingFile := result.Files[2]
	assert.Equal(t, "k8s/web-ingress.yaml", ingFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(ingFile.Content, &doc))
	assert.Equal(t, "networking.k8s.io/v1", doc["apiVersion"])
	assert.Equal(t, "Ingress", doc["kind"])
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

func TestCompiler_Compile_ServiceAPIVersion(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "svcver",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)
	require.Len(t, result.Files, 2)

	// Check service.
	svcFile := result.Files[1]
	assert.Equal(t, "k8s/api-service.yaml", svcFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(svcFile.Content, &doc))
	assert.Equal(t, "v1", doc["apiVersion"])
	assert.Equal(t, "Service", doc["kind"])

	spec := doc["spec"].(map[string]any)
	assert.Equal(t, "ClusterIP", spec["type"])
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

func TestCompiler_Compile_AllProbes(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "probeall",
		Slices: []plan.Slice{
			{
				Name:       "api",
				Kind:       plan.SliceKindWeb,
				Image:      "myorg/api:1.0",
				Port:       8080,
				Replicas:   1,
				HealthPath: "/healthz",
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

	// Liveness probe.
	liveness, ok := container["livenessProbe"].(map[string]any)
	require.True(t, ok, "livenessProbe should be present")
	livenessHTTP := liveness["httpGet"].(map[string]any)
	assert.Equal(t, "/healthz", livenessHTTP["path"])
	assert.Equal(t, 8080, livenessHTTP["port"])

	// Readiness probe.
	readiness, ok := container["readinessProbe"].(map[string]any)
	require.True(t, ok, "readinessProbe should be present")
	readinessHTTP := readiness["httpGet"].(map[string]any)
	assert.Equal(t, "/healthz", readinessHTTP["path"])
	assert.Equal(t, 8080, readinessHTTP["port"])

	// Startup probe.
	startup, ok := container["startupProbe"].(map[string]any)
	require.True(t, ok, "startupProbe should be present")
	startupHTTP := startup["httpGet"].(map[string]any)
	assert.Equal(t, "/healthz", startupHTTP["path"])
	assert.Equal(t, 8080, startupHTTP["port"])
}

func TestCompiler_Compile_Namespace(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name:      "nstest",
		Namespace: "staging",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Public:   true,
				Replicas: 1,
			},
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

	// All manifests should have namespace set.
	for _, f := range result.Files {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(f.Content, &doc), "invalid YAML in %s", f.Path)

		meta, ok := doc["metadata"].(map[string]any)
		require.True(t, ok, "metadata should exist in %s", f.Path)
		assert.Equal(t, "staging", meta["namespace"], "namespace should be staging in %s", f.Path)
	}
}

func TestCompiler_Compile_DomainIngress(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "domaintest",
		Slices: []plan.Slice{
			{
				Name:     "web",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/web:1.0",
				Port:     3000,
				Public:   true,
				Replicas: 1,
				Domain:   "app.example.com",
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Ingress is the last file: deployment, service, ingress.
	require.Len(t, result.Files, 3)
	ingFile := result.Files[2]
	assert.Equal(t, "k8s/web-ingress.yaml", ingFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(ingFile.Content, &doc))

	spec := doc["spec"].(map[string]any)
	rules := spec["rules"].([]any)
	require.Len(t, rules, 1)

	rule := rules[0].(map[string]any)
	assert.Equal(t, "app.example.com", rule["host"], "host should match domain")
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

func TestCompiler_Compile_HeadlessService(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "headlesstest",
		Slices: []plan.Slice{
			{
				Name:            "zk",
				Kind:            plan.SliceKindStateful,
				Image:           "zookeeper:3.8",
				Port:            2181,
				Replicas:        3,
				StatefulStorage: "5Gi",
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// StatefulSet + headless Service.
	require.Len(t, result.Files, 2)
	svcFile := result.Files[1]
	assert.Equal(t, "k8s/zk-service.yaml", svcFile.Path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(svcFile.Content, &doc))
	assert.Equal(t, "Service", doc["kind"])

	spec := doc["spec"].(map[string]any)
	assert.Equal(t, "None", spec["clusterIP"], "headless service should have clusterIP: None")
}

func TestCompiler_Compile_MultiPortService(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "multiport",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Replicas: 1,
				Ports: []plan.PortSpec{
					{Name: "http", Port: 8080, Protocol: "tcp"},
					{Name: "grpc", Port: 9090, Protocol: "tcp"},
					{Name: "metrics", Port: 9100, Protocol: "tcp"},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// Deployment + Service (Ports[] makes hasPort true).
	require.Len(t, result.Files, 2)

	// Check deployment container ports.
	var depDoc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &depDoc))
	spec := depDoc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	container := containers[0].(map[string]any)
	ports := container["ports"].([]any)
	require.Len(t, ports, 3)
	assert.Equal(t, "http", ports[0].(map[string]any)["name"])
	assert.Equal(t, 8080, ports[0].(map[string]any)["containerPort"])
	assert.Equal(t, "grpc", ports[1].(map[string]any)["name"])
	assert.Equal(t, 9090, ports[1].(map[string]any)["containerPort"])

	// Check service ports.
	var svcDoc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[1].Content, &svcDoc))
	svcSpec := svcDoc["spec"].(map[string]any)
	svcPorts := svcSpec["ports"].([]any)
	require.Len(t, svcPorts, 3)
	assert.Equal(t, "http", svcPorts[0].(map[string]any)["name"])
	assert.Equal(t, 8080, svcPorts[0].(map[string]any)["port"])
	assert.Equal(t, "grpc", svcPorts[1].(map[string]any)["name"])
}

func TestCompiler_Compile_TCPProbe(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "tcpprobe",
		Slices: []plan.Slice{
			{
				Name:     "db",
				Kind:     plan.SliceKindCache,
				Image:    "redis:7",
				Port:     6379,
				Replicas: 1,
				Probes: []plan.ProbeSpec{
					{
						Type:     "readiness",
						TCPPort:  6379,
						Interval: 10,
						Timeout:  5,
						Delay:    15,
					},
					{
						Type:    "liveness",
						TCPPort: 6379,
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
	container := containers[0].(map[string]any)

	// Readiness probe should be TCP.
	readiness := container["readinessProbe"].(map[string]any)
	tcpSocket := readiness["tcpSocket"].(map[string]any)
	assert.Equal(t, 6379, tcpSocket["port"])
	assert.Equal(t, 10, readiness["periodSeconds"])
	assert.Equal(t, 5, readiness["timeoutSeconds"])
	assert.Equal(t, 15, readiness["initialDelaySeconds"])

	// Liveness probe should also be TCP.
	liveness := container["livenessProbe"].(map[string]any)
	_, hasTCP := liveness["tcpSocket"]
	assert.True(t, hasTCP)
}

func TestCompiler_Compile_ExecProbe(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "execprobe",
		Slices: []plan.Slice{
			{
				Name:     "worker",
				Kind:     plan.SliceKindWorker,
				Image:    "myorg/worker:1.0",
				Replicas: 1,
				Probes: []plan.ProbeSpec{
					{
						Type:    "liveness",
						Command: "/bin/sh -c health-check",
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
	container := containers[0].(map[string]any)

	liveness := container["livenessProbe"].(map[string]any)
	exec := liveness["exec"].(map[string]any)
	command := exec["command"].([]any)
	require.Len(t, command, 3)
	assert.Equal(t, "/bin/sh", command[0])
	assert.Equal(t, "-c", command[1])
	assert.Equal(t, "health-check", command[2])
}

func TestCompiler_Compile_HTTPProbeWithTiming(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "httpprobe",
		Slices: []plan.Slice{
			{
				Name:     "api",
				Kind:     plan.SliceKindWeb,
				Image:    "myorg/api:1.0",
				Port:     8080,
				Replicas: 1,
				Probes: []plan.ProbeSpec{
					{
						Type:     "readiness",
						HTTPPath: "/ready",
						TCPPort:  8080,
						Interval: 15,
						Timeout:  3,
						Delay:    10,
					},
					{
						Type:     "liveness",
						HTTPPath: "/healthz",
						TCPPort:  8080,
					},
					{
						Type:     "startup",
						HTTPPath: "/healthz",
						TCPPort:  8080,
						Delay:    30,
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
	container := containers[0].(map[string]any)

	readiness := container["readinessProbe"].(map[string]any)
	httpGet := readiness["httpGet"].(map[string]any)
	assert.Equal(t, "/ready", httpGet["path"])
	assert.Equal(t, 8080, httpGet["port"])
	assert.Equal(t, 15, readiness["periodSeconds"])
	assert.Equal(t, 3, readiness["timeoutSeconds"])
	assert.Equal(t, 10, readiness["initialDelaySeconds"])

	startup := container["startupProbe"].(map[string]any)
	assert.Equal(t, 30, startup["initialDelaySeconds"])
}

func TestCompiler_Compile_BackwardCompatWebDeployment(t *testing.T) {
	t.Parallel()

	// Ensure web and worker slices still generate Deployments (backward compat).
	p := &plan.AppPlan{
		Name: "compat",
		Slices: []plan.Slice{
			{
				Name:       "web",
				Kind:       plan.SliceKindWeb,
				Image:      "myorg/web:1.0",
				Port:       3000,
				Replicas:   2,
				HealthPath: "/health",
			},
			{
				Name:     "worker",
				Kind:     plan.SliceKindWorker,
				Image:    "myorg/worker:1.0",
				Replicas: 3,
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// web: deployment + service = 2, worker: deployment = 1.
	require.Len(t, result.Files, 3)
	assert.Equal(t, "k8s/web-deployment.yaml", result.Files[0].Path)
	assert.Equal(t, "k8s/worker-deployment.yaml", result.Files[1].Path)
	assert.Equal(t, "k8s/web-service.yaml", result.Files[2].Path)

	// Verify web still has HealthPath-based probes.
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &doc))
	spec := doc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	container := containers[0].(map[string]any)

	_, hasReadiness := container["readinessProbe"]
	assert.True(t, hasReadiness, "web slice with HealthPath should have readinessProbe")
	_, hasLiveness := container["livenessProbe"]
	assert.True(t, hasLiveness, "web slice with HealthPath should have livenessProbe")
}

func TestCompiler_Compile_UDPPort(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "udptest",
		Slices: []plan.Slice{
			{
				Name:     "dns",
				Kind:     plan.SliceKindDaemon,
				Image:    "myorg/dns:1.0",
				Replicas: 1,
				Ports: []plan.PortSpec{
					{Name: "dns-udp", Port: 53, Protocol: "udp"},
					{Name: "dns-tcp", Port: 53, Protocol: "tcp"},
				},
			},
		},
	}

	c := k8s.New()
	result, err := c.Compile(context.Background(), p)
	require.NoError(t, err)

	// DaemonSet + Service (daemon with ports gets a service).
	require.Len(t, result.Files, 2)

	var svcDoc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Files[1].Content, &svcDoc))
	svcSpec := svcDoc["spec"].(map[string]any)
	svcPorts := svcSpec["ports"].([]any)
	require.Len(t, svcPorts, 2)
	assert.Equal(t, "UDP", svcPorts[0].(map[string]any)["protocol"])
	assert.Equal(t, "TCP", svcPorts[1].(map[string]any)["protocol"])
}
