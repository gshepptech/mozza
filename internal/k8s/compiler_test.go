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
			// db:       statefulset + headless service = 2 (no PVC; uses volumeClaimTemplates)
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
