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

// composeFile mirrors the internal compose struct for test unmarshaling.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]struct{}       `yaml:"volumes,omitempty"`
	Networks map[string]composeNetwork `yaml:"networks,omitempty"`
}

// composeService mirrors the internal service struct for test unmarshaling.
type composeService struct {
	Image           string            `yaml:"image"`
	Command         string            `yaml:"command,omitempty"`
	User            string            `yaml:"user,omitempty"`
	ReadOnly        bool              `yaml:"read_only,omitempty"`
	Ports           []string          `yaml:"ports,omitempty"`
	Expose          []string          `yaml:"expose,omitempty"`
	Environment     map[string]string `yaml:"environment,omitempty"`
	DependsOn       interface{}       `yaml:"depends_on,omitempty"`
	Volumes         []string          `yaml:"volumes,omitempty"`
	Networks        []string          `yaml:"networks,omitempty"`
	NetworkMode     string            `yaml:"network_mode,omitempty"`
	Restart         string            `yaml:"restart,omitempty"`
	Deploy          *composeDeploy    `yaml:"deploy,omitempty"`
	Healthcheck     *composeHealth    `yaml:"healthcheck,omitempty"`
	Labels          map[string]string `yaml:"labels,omitempty"`
	CapDrop         []string          `yaml:"cap_drop,omitempty"`
	CapAdd          []string          `yaml:"cap_add,omitempty"`
	StopGracePeriod string            `yaml:"stop_grace_period,omitempty"`
}

// dependsOnList extracts depends_on as a string slice (for simple cases).
func dependsOnList(dep interface{}) []string {
	if dep == nil {
		return nil
	}
	switch v := dep.(type) {
	case []interface{}:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	}
	return nil
}

// dependsOnMap extracts depends_on as a map with conditions (for structured cases).
func dependsOnMap(dep interface{}) map[string]string {
	if dep == nil {
		return nil
	}
	m, ok := dep.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		if inner, ok := v.(map[string]interface{}); ok {
			if cond, ok := inner["condition"].(string); ok {
				result[k] = cond
			}
		}
	}
	return result
}

// composeDeploy mirrors the internal deploy struct for test unmarshaling.
type composeDeploy struct {
	Replicas  int                    `yaml:"replicas,omitempty"`
	Resources *composeDeployResource `yaml:"resources,omitempty"`
}

// composeDeployResource mirrors the internal deploy resources struct for test unmarshaling.
type composeDeployResource struct {
	Limits composeLimits `yaml:"limits"`
}

// composeLimits holds CPU and memory limits for test unmarshaling.
type composeLimits struct {
	CPUs   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// composeHealth mirrors the internal healthcheck struct for test unmarshaling.
type composeHealth struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}

// composeNetwork mirrors the internal network struct for test unmarshaling.
type composeNetwork struct {
	Driver string `yaml:"driver"`
}

func TestCompiler_Name(t *testing.T) {
	t.Parallel()

	c := local.New()
	assert.Equal(t, "local", c.Name())
}

func TestCompiler_Compile_NilPlan(t *testing.T) {
	t.Parallel()

	c := local.New()
	_, err := c.Compile(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be nil")
}

func TestCompiler_Compile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		plan    *plan.AppPlan
		assertF func(t *testing.T, cf composeFile, summary string, warnings []string)
	}{
		{
			name: "simple API with database",
			plan: &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:       "api",
						Kind:       plan.SliceKindWeb,
						Image:      "myapp-api:latest",
						Port:       8080,
						Public:     true,
						HealthPath: "/healthz",
						Needs:      []string{"db"},
					},
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
						Database: &plan.DatabaseSpec{
							Storage: "10Gi",
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, summary string, warnings []string) {
				t.Helper()

				require.Len(t, cf.Services, 2)

				// API service checks.
				api := cf.Services["api"]
				assert.Equal(t, "myapp-api:latest", api.Image)
				assert.Equal(t, []string{"8080:8080"}, api.Ports)
				assert.Equal(t, []string{"db"}, dependsOnList(api.DependsOn))
				require.NotNil(t, api.Healthcheck)
				assert.Equal(t, []string{"CMD", "curl", "-f", "http://localhost:8080/healthz"}, api.Healthcheck.Test)

				// DB service checks.
				db := cf.Services["db"]
				assert.Equal(t, "postgres:16", db.Image)
				assert.Equal(t, []string{"5432:5432"}, db.Ports)
				assert.Equal(t, []string{"db-data:/var/lib/data"}, db.Volumes)

				// Volume checks.
				_, hasVol := cf.Volumes["db-data"]
				assert.True(t, hasVol, "expected db-data volume")

				// Network checks.
				net, hasNet := cf.Networks["myapp-net"]
				assert.True(t, hasNet, "expected myapp-net network")
				assert.Equal(t, "bridge", net.Driver)

				assert.Contains(t, summary, "myapp")
				assert.Contains(t, summary, "2 service(s)")
				assert.Empty(t, warnings)
			},
		},
		{
			name: "public web service exposes port",
			plan: &plan.AppPlan{
				Name: "webapp",
				Slices: []plan.Slice{
					{
						Name:   "frontend",
						Kind:   plan.SliceKindWeb,
						Image:  "nginx:latest",
						Port:   3000,
						Public: true,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				fe := cf.Services["frontend"]
				assert.Equal(t, []string{"3000:3000"}, fe.Ports)
			},
		},
		{
			name: "non-public web service has no ports",
			plan: &plan.AppPlan{
				Name: "internal",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				assert.Empty(t, api.Ports, "non-public web should not expose ports")
			},
		},
		{
			name: "worker slice has no ports",
			plan: &plan.AppPlan{
				Name: "bgapp",
				Slices: []plan.Slice{
					{
						Name:  "worker",
						Kind:  plan.SliceKindWorker,
						Image: "worker:latest",
						Port:  0,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				w := cf.Services["worker"]
				assert.Empty(t, w.Ports, "worker should not expose ports")
				assert.Nil(t, w.Healthcheck, "worker without health path should have no healthcheck")
			},
		},
		{
			name: "full app with all slice kinds",
			plan: &plan.AppPlan{
				Name: "fullapp",
				Slices: []plan.Slice{
					{
						Name:       "web",
						Kind:       plan.SliceKindWeb,
						Image:      "web:latest",
						Port:       3000,
						Public:     true,
						HealthPath: "/health",
						Needs:      []string{"api"},
					},
					{
						Name:       "api",
						Kind:       plan.SliceKindWeb,
						Image:      "api:latest",
						Port:       8080,
						Public:     false,
						HealthPath: "/healthz",
						Needs:      []string{"db", "cache"},
					},
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
						Database: &plan.DatabaseSpec{
							Storage: "20Gi",
						},
					},
					{
						Name:  "cache",
						Kind:  plan.SliceKindCache,
						Image: "redis:7",
						Port:  6379,
						Cache: &plan.CacheSpec{
							Storage: "1Gi",
						},
					},
					{
						Name:     "worker",
						Kind:     plan.SliceKindWorker,
						Image:    "worker:latest",
						Replicas: 3,
						Needs:    []string{"db", "cache"},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, summary string, _ []string) {
				t.Helper()

				require.Len(t, cf.Services, 5)

				// Web (public).
				web := cf.Services["web"]
				assert.Equal(t, []string{"3000:3000"}, web.Ports)
				assert.Equal(t, []string{"api"}, dependsOnList(web.DependsOn))
				require.NotNil(t, web.Healthcheck)

				// API (non-public).
				api := cf.Services["api"]
				assert.Empty(t, api.Ports, "non-public API should not expose ports")
				assert.Equal(t, []string{"db", "cache"}, dependsOnList(api.DependsOn))
				require.NotNil(t, api.Healthcheck)

				// Database.
				db := cf.Services["db"]
				assert.Equal(t, []string{"5432:5432"}, db.Ports)
				assert.Equal(t, []string{"db-data:/var/lib/data"}, db.Volumes)

				// Cache.
				cache := cf.Services["cache"]
				assert.Equal(t, []string{"6379:6379"}, cache.Ports)
				assert.Equal(t, []string{"cache-data:/data"}, cache.Volumes)

				// Worker.
				worker := cf.Services["worker"]
				assert.Empty(t, worker.Ports)
				require.NotNil(t, worker.Deploy)
				assert.Equal(t, 3, worker.Deploy.Replicas)
				assert.Equal(t, []string{"db", "cache"}, dependsOnList(worker.DependsOn))

				// Volumes.
				_, hasDBVol := cf.Volumes["db-data"]
				assert.True(t, hasDBVol)
				_, hasCacheVol := cf.Volumes["cache-data"]
				assert.True(t, hasCacheVol)

				// Network.
				_, hasNet := cf.Networks["fullapp-net"]
				assert.True(t, hasNet)

				assert.Contains(t, summary, "5 service(s)")
				assert.Contains(t, summary, "2 volume(s)")
				assert.Contains(t, summary, "1 network(s)")
			},
		},
		{
			name: "YAML round-trips through compose structure",
			plan: &plan.AppPlan{
				Name: "roundtrip",
				Slices: []plan.Slice{
					{
						Name:   "web",
						Kind:   plan.SliceKindWeb,
						Image:  "web:latest",
						Port:   8080,
						Public: true,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				// Re-marshal and unmarshal to verify YAML validity.
				data, err := yaml.Marshal(cf)
				require.NoError(t, err)

				var roundTripped composeFile
				require.NoError(t, yaml.Unmarshal(data, &roundTripped))

				assert.Equal(t, cf.Services["web"].Image, roundTripped.Services["web"].Image)
				assert.Equal(t, cf.Services["web"].Ports, roundTripped.Services["web"].Ports)
			},
		},
		{
			name: "depends_on matches Needs",
			plan: &plan.AppPlan{
				Name: "depcheck",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Needs: []string{"db", "cache", "queue"},
					},
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
					},
					{
						Name:  "cache",
						Kind:  plan.SliceKindCache,
						Image: "redis:7",
						Port:  6379,
					},
					{
						Name:  "queue",
						Kind:  plan.SliceKindWorker,
						Image: "rabbitmq:3",
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				assert.Equal(t, []string{"db", "cache", "queue"}, dependsOnList(api.DependsOn))
			},
		},
		{
			name: "database without storage has no volume",
			plan: &plan.AppPlan{
				Name: "novol",
				Slices: []plan.Slice{
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				db := cf.Services["db"]
				assert.Empty(t, db.Volumes, "database without storage should have no volume mount")
				assert.Empty(t, cf.Volumes, "no named volumes expected")
			},
		},
		{
			name: "healthcheck generated when HealthPath present",
			plan: &plan.AppPlan{
				Name: "hcapp",
				Slices: []plan.Slice{
					{
						Name:       "api",
						Kind:       plan.SliceKindWeb,
						Image:      "api:latest",
						Port:       9090,
						HealthPath: "/ready",
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				require.NotNil(t, api.Healthcheck)
				assert.Equal(t, []string{"CMD", "curl", "-f", "http://localhost:9090/ready"}, api.Healthcheck.Test)
				assert.Equal(t, "10s", api.Healthcheck.Interval)
				assert.Equal(t, "5s", api.Healthcheck.Timeout)
				assert.Equal(t, 3, api.Healthcheck.Retries)
			},
		},
		{
			name: "no healthcheck when HealthPath empty",
			plan: &plan.AppPlan{
				Name: "nohc",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				assert.Nil(t, api.Healthcheck, "no healthcheck expected without HealthPath")
			},
		},
		{
			name: "network name derived from app name",
			plan: &plan.AppPlan{
				Name: "my-cool-app",
				Slices: []plan.Slice{
					{
						Name:  "svc",
						Kind:  plan.SliceKindWeb,
						Image: "svc:latest",
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				_, hasNet := cf.Networks["my-cool-app-net"]
				assert.True(t, hasNet, "network should be named my-cool-app-net")

				svc := cf.Services["svc"]
				assert.Equal(t, []string{"my-cool-app-net"}, svc.Networks)
			},
		},
		{
			name: "empty slices produces empty services",
			plan: &plan.AppPlan{
				Name:   "empty",
				Slices: nil,
			},
			assertF: func(t *testing.T, cf composeFile, summary string, _ []string) {
				t.Helper()

				assert.Empty(t, cf.Services)
				_, hasNet := cf.Networks["empty-net"]
				assert.True(t, hasNet)
				assert.Contains(t, summary, "0 service(s)")
			},
		},
		{
			name: "service with env vars appears in compose output",
			plan: &plan.AppPlan{
				Name: "envapp",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Env: map[string]string{
							"DATABASE_URL": "postgres://localhost/db",
							"LOG_LEVEL":    "debug",
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				require.NotNil(t, api.Environment)
				assert.Equal(t, "postgres://localhost/db", api.Environment["DATABASE_URL"])
				assert.Equal(t, "debug", api.Environment["LOG_LEVEL"])
			},
		},
		{
			name: "service with restart policy appears in compose output",
			plan: &plan.AppPlan{
				Name: "restartapp",
				Slices: []plan.Slice{
					{
						Name:          "api",
						Kind:          plan.SliceKindWeb,
						Image:         "api:latest",
						Port:          8080,
						RestartPolicy: "unless-stopped",
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				assert.Equal(t, "unless-stopped", api.Restart)
			},
		},
		{
			name: "service with resource limits appears in compose output",
			plan: &plan.AppPlan{
				Name: "limitsapp",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Resources: &plan.ResourceSpec{
							CPULimit:    "500m",
							MemoryLimit: "256Mi",
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				api := cf.Services["api"]
				require.NotNil(t, api.Deploy, "deploy section should be present for resource limits")
				require.NotNil(t, api.Deploy.Resources, "resources should be present in deploy")
				assert.Equal(t, "0.5", api.Deploy.Resources.Limits.CPUs)
				assert.Equal(t, "256M", api.Deploy.Resources.Limits.Memory)
			},
		},
		{
			name: "service with MountPath uses correct mount path",
			plan: &plan.AppPlan{
				Name: "mountapp",
				Slices: []plan.Slice{
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
						Database: &plan.DatabaseSpec{
							Storage:   "10Gi",
							MountPath: "/var/lib/postgresql/data",
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				db := cf.Services["db"]
				require.Len(t, db.Volumes, 1)
				assert.Equal(t, "db-data:/var/lib/postgresql/data", db.Volumes[0])
			},
		},
		// --- New tests for expanded local compiler ---
		{
			name: "multi-port service generates multiple port entries",
			plan: &plan.AppPlan{
				Name: "multiport",
				Slices: []plan.Slice{
					{
						Name:   "gateway",
						Kind:   plan.SliceKindGateway,
						Image:  "envoy:latest",
						Public: true,
						Ports: []plan.PortSpec{
							{Name: "http", Port: 8080},
							{Name: "grpc", Port: 9090},
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				gw := cf.Services["gateway"]
				assert.Equal(t, []string{"8080:8080", "9090:9090"}, gw.Ports)
			},
		},
		{
			name: "API kind uses expose instead of ports",
			plan: &plan.AppPlan{
				Name: "apiapp",
				Slices: []plan.Slice{
					{
						Name:  "backend",
						Kind:  plan.SliceKindAPI,
						Image: "backend:latest",
						Port:  8080,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				svc := cf.Services["backend"]
				assert.Empty(t, svc.Ports, "API should not have host-mapped ports")
				assert.Equal(t, []string{"8080"}, svc.Expose)
			},
		},
		{
			name: "task kind gets restart no",
			plan: &plan.AppPlan{
				Name: "taskapp",
				Slices: []plan.Slice{
					{
						Name:  "migrate",
						Kind:  plan.SliceKindTask,
						Image: "flyway:latest",
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				svc := cf.Services["migrate"]
				assert.Equal(t, "no", svc.Restart)
				assert.Empty(t, svc.Ports)
			},
		},
		{
			name: "scheduled kind gets restart no and schedule label and warning",
			plan: &plan.AppPlan{
				Name: "cronapp",
				Slices: []plan.Slice{
					{
						Name:     "cleanup",
						Kind:     plan.SliceKindScheduled,
						Image:    "cleanup:latest",
						Schedule: "0 */6 * * *",
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, warnings []string) {
				t.Helper()

				svc := cf.Services["cleanup"]
				assert.Equal(t, "no", svc.Restart)
				assert.Equal(t, "0 */6 * * *", svc.Labels["mozza.schedule"])
				require.Len(t, warnings, 1)
				assert.Contains(t, warnings[0], "Scheduled tasks not supported")
			},
		},
		{
			name: "stateful kind gets named volume and warning for peer discovery",
			plan: &plan.AppPlan{
				Name: "statefulapp",
				Slices: []plan.Slice{
					{
						Name:            "store",
						Kind:            plan.SliceKindStateful,
						Image:           "etcd:latest",
						Port:            2379,
						StatefulStorage: "5Gi",
						PeerDiscovery:   true,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, warnings []string) {
				t.Helper()

				svc := cf.Services["store"]
				assert.Equal(t, []string{"2379:2379"}, svc.Ports)
				assert.Contains(t, svc.Volumes, "store-data:/data")

				_, hasVol := cf.Volumes["store-data"]
				assert.True(t, hasVol, "expected store-data volume")

				require.Len(t, warnings, 1)
				assert.Contains(t, warnings[0], "Headless services are Kubernetes-only")
			},
		},
		{
			name: "gateway kind with public port",
			plan: &plan.AppPlan{
				Name: "gwapp",
				Slices: []plan.Slice{
					{
						Name:   "ingress",
						Kind:   plan.SliceKindGateway,
						Image:  "traefik:latest",
						Port:   80,
						Public: true,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				svc := cf.Services["ingress"]
				assert.Equal(t, []string{"80:80"}, svc.Ports)
			},
		},
		{
			name: "daemon kind gets label and warning",
			plan: &plan.AppPlan{
				Name: "daemonapp",
				Slices: []plan.Slice{
					{
						Name:       "agent",
						Kind:       plan.SliceKindDaemon,
						Image:      "agent:latest",
						DaemonMode: true,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, warnings []string) {
				t.Helper()

				svc := cf.Services["agent"]
				assert.Equal(t, "true", svc.Labels["mozza.daemon-mode"])
				assert.Empty(t, svc.Ports)

				require.Len(t, warnings, 1)
				assert.Contains(t, warnings[0], "DaemonSet mode requires Docker Swarm")
			},
		},
		{
			name: "init containers generate separate services with depends_on",
			plan: &plan.AppPlan{
				Name: "initapp",
				Slices: []plan.Slice{
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
						Database: &plan.DatabaseSpec{
							Storage: "10Gi",
						},
					},
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Needs: []string{"db"},
						InitSteps: []plan.InitStep{
							{
								Image:   "flyway/flyway:latest",
								Command: "migrate",
							},
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				// Should have 3 services: db, api, api-init-0.
				require.Len(t, cf.Services, 3)

				// Init service.
				initSvc := cf.Services["api-init-0"]
				assert.Equal(t, "flyway/flyway:latest", initSvc.Image)
				assert.Equal(t, "migrate", initSvc.Command)
				assert.Equal(t, "no", initSvc.Restart)

				// Init service depends on db with service_healthy.
				initDeps := dependsOnMap(initSvc.DependsOn)
				require.NotNil(t, initDeps)
				assert.Equal(t, "service_healthy", initDeps["db"])

				// Main service depends on init with service_completed_successfully.
				apiDeps := dependsOnMap(cf.Services["api"].DependsOn)
				require.NotNil(t, apiDeps)
				assert.Equal(t, "service_completed_successfully", apiDeps["api-init-0"])
				assert.Equal(t, "service_started", apiDeps["db"])
			},
		},
		{
			name: "sidecar generates separate service with network_mode",
			plan: &plan.AppPlan{
				Name: "sidecarapp",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Sidecars: []plan.Sidecar{
							{
								Name:  "envoy",
								Image: "envoyproxy/envoy:v1.28",
								Ports: []plan.PortSpec{{Name: "admin", Port: 9901}},
							},
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				require.Len(t, cf.Services, 2)

				sidecar := cf.Services["api-envoy"]
				assert.Equal(t, "envoyproxy/envoy:v1.28", sidecar.Image)
				assert.Equal(t, "service:api", sidecar.NetworkMode)
				assert.Equal(t, []string{"api"}, dependsOnList(sidecar.DependsOn))
				assert.Equal(t, []string{"9901:9901"}, sidecar.Ports)
			},
		},
		{
			name: "file mounts generate bind mount volumes",
			plan: &plan.AppPlan{
				Name: "mountsapp",
				Slices: []plan.Slice{
					{
						Name:  "web",
						Kind:  plan.SliceKindWeb,
						Image: "nginx:latest",
						Port:  80,
						Mounts: []plan.MountSpec{
							{
								Source:   "./nginx.conf",
								Target:   "/etc/nginx/nginx.conf",
								ReadOnly: true,
							},
							{
								Source: "./html",
								Target: "/usr/share/nginx/html",
							},
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				svc := cf.Services["web"]
				require.Len(t, svc.Volumes, 2)
				assert.Equal(t, "./nginx.conf:/etc/nginx/nginx.conf:ro", svc.Volumes[0])
				assert.Equal(t, "./html:/usr/share/nginx/html", svc.Volumes[1])
			},
		},
		{
			name: "security context maps to compose fields",
			plan: &plan.AppPlan{
				Name: "secapp",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Security: &plan.SecuritySpec{
							RunAsUser:        1000,
							ReadOnlyRoot:     true,
							DropCapabilities: []string{"ALL"},
							AddCapabilities:  []string{"NET_BIND_SERVICE"},
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				svc := cf.Services["api"]
				assert.Equal(t, "1000", svc.User)
				assert.True(t, svc.ReadOnly)
				assert.Equal(t, []string{"ALL"}, svc.CapDrop)
				assert.Equal(t, []string{"NET_BIND_SERVICE"}, svc.CapAdd)
			},
		},
		{
			name: "graceful shutdown maps to stop_grace_period",
			plan: &plan.AppPlan{
				Name: "graceapp",
				Slices: []plan.Slice{
					{
						Name:             "api",
						Kind:             plan.SliceKindWeb,
						Image:            "api:latest",
						Port:             8080,
						GracefulShutdown: 60,
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, _ []string) {
				t.Helper()

				svc := cf.Services["api"]
				assert.Equal(t, "60s", svc.StopGracePeriod)
			},
		},
		{
			name: "K8s-only features generate warnings",
			plan: &plan.AppPlan{
				Name: "warnapp",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						Permissions: []plan.Permission{
							{Verbs: []string{"get"}, Resources: []string{"pods"}},
						},
						NetworkPolicy: &plan.NetworkPolicySpec{DenyAll: true},
						Scheduling:    &plan.SchedulingSpec{AntiAffinity: true},
						AutoScale: &plan.AutoScaleSpec{
							MinReplicas: 2,
							MaxReplicas: 10,
							CPUTarget:   80,
						},
						DisruptionBudget: &plan.DisruptionBudgetSpec{MinAvailable: 1},
						UpdateStrategy:   &plan.UpdateStrategySpec{MaxSurge: "25%"},
					},
				},
			},
			assertF: func(t *testing.T, _ composeFile, _ string, warnings []string) {
				t.Helper()

				require.Len(t, warnings, 6)
				assert.Contains(t, warnings[0], "Permissions are Kubernetes-only")
				assert.Contains(t, warnings[1], "Network policies are Kubernetes-only")
				assert.Contains(t, warnings[2], "Node scheduling is Kubernetes-only")
				assert.Contains(t, warnings[3], "Auto-scaling is Kubernetes-only")
				assert.Contains(t, warnings[4], "Disruption budgets are Kubernetes-only")
				assert.Contains(t, warnings[5], "Update strategy is Kubernetes-only")
			},
		},
		{
			name: "auto-scale uses MinReplicas as local replica count",
			plan: &plan.AppPlan{
				Name: "hpaapp",
				Slices: []plan.Slice{
					{
						Name:  "api",
						Kind:  plan.SliceKindWeb,
						Image: "api:latest",
						Port:  8080,
						AutoScale: &plan.AutoScaleSpec{
							MinReplicas: 3,
							MaxReplicas: 10,
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, warnings []string) {
				t.Helper()

				svc := cf.Services["api"]
				require.NotNil(t, svc.Deploy)
				assert.Equal(t, 3, svc.Deploy.Replicas)
				require.Len(t, warnings, 1)
				assert.Contains(t, warnings[0], "Auto-scaling is Kubernetes-only")
			},
		},
		{
			name: "backward compat: simple recipe produces identical output",
			plan: &plan.AppPlan{
				Name: "compat",
				Slices: []plan.Slice{
					{
						Name:   "web",
						Kind:   plan.SliceKindWeb,
						Image:  "web:latest",
						Port:   3000,
						Public: true,
					},
					{
						Name:  "db",
						Kind:  plan.SliceKindDatabase,
						Image: "postgres:16",
						Port:  5432,
						Database: &plan.DatabaseSpec{
							Storage: "10Gi",
						},
					},
				},
			},
			assertF: func(t *testing.T, cf composeFile, _ string, warnings []string) {
				t.Helper()

				// No warnings for simple plans.
				assert.Empty(t, warnings)

				// Same structure as before.
				require.Len(t, cf.Services, 2)
				web := cf.Services["web"]
				assert.Equal(t, "web:latest", web.Image)
				assert.Equal(t, []string{"3000:3000"}, web.Ports)
				assert.Empty(t, web.Expose)
				assert.Empty(t, web.CapDrop)
				assert.Empty(t, web.CapAdd)
				assert.Empty(t, web.User)
				assert.False(t, web.ReadOnly)
				assert.Empty(t, web.StopGracePeriod)
				assert.Empty(t, web.NetworkMode)

				db := cf.Services["db"]
				assert.Equal(t, []string{"5432:5432"}, db.Ports)
				assert.Equal(t, []string{"db-data:/var/lib/data"}, db.Volumes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := local.New()
			result, err := c.Compile(context.Background(), tt.plan)
			require.NoError(t, err)
			require.Len(t, result.Files, 1)
			assert.Equal(t, "docker-compose.yml", result.Files[0].Path)

			var cf composeFile
			require.NoError(t, yaml.Unmarshal(result.Files[0].Content, &cf))

			tt.assertF(t, cf, result.Summary, result.Warnings)
		})
	}
}
