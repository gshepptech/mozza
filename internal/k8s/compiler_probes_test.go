package k8s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
)

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

func TestBuildDeployment_ProbePortResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		slice   plan.Slice
		assertF func(t *testing.T, c corev1.Container)
	}{
		{
			name: "http probes inherit the slice port",
			slice: plan.Slice{
				Name:  "web",
				Kind:  plan.SliceKindWeb,
				Image: "ghost:5",
				Port:  2368,
				Probes: []plan.ProbeSpec{
					{Type: "readiness", HTTPPath: "/ready"},
					{Type: "liveness", HTTPPath: "/healthz"},
					{Type: "startup", HTTPPath: "/healthz"},
				},
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				require.NotNil(t, c.ReadinessProbe)
				require.NotNil(t, c.ReadinessProbe.HTTPGet)
				assert.Equal(t, "/ready", c.ReadinessProbe.HTTPGet.Path)
				assert.Equal(t, int32(2368), c.ReadinessProbe.HTTPGet.Port.IntVal)

				require.NotNil(t, c.LivenessProbe)
				require.NotNil(t, c.LivenessProbe.HTTPGet)
				assert.Equal(t, int32(2368), c.LivenessProbe.HTTPGet.Port.IntVal)

				require.NotNil(t, c.StartupProbe)
				require.NotNil(t, c.StartupProbe.HTTPGet)
				assert.Equal(t, int32(2368), c.StartupProbe.HTTPGet.Port.IntVal)
			},
		},
		{
			name: "http probe inherits a named port when there is no bare port",
			slice: plan.Slice{
				Name:   "web",
				Kind:   plan.SliceKindWeb,
				Image:  "gitea/gitea:1",
				Ports:  []plan.PortSpec{{Name: "http", Port: 3000}, {Name: "ssh", Port: 22}},
				Probes: []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready"}},
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				require.NotNil(t, c.ReadinessProbe)
				require.NotNil(t, c.ReadinessProbe.HTTPGet)
				assert.Equal(t, int32(3000), c.ReadinessProbe.HTTPGet.Port.IntVal)
			},
		},
		{
			name: "a probe's own port wins over the slice port",
			slice: plan.Slice{
				Name:   "web",
				Kind:   plan.SliceKindWeb,
				Image:  "myorg/api:1.0",
				Port:   8080,
				Probes: []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready", TCPPort: 9000}},
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				require.NotNil(t, c.ReadinessProbe)
				require.NotNil(t, c.ReadinessProbe.HTTPGet)
				assert.Equal(t, int32(9000), c.ReadinessProbe.HTTPGet.Port.IntVal)
			},
		},
		{
			name: "no resolvable port emits no probe",
			slice: plan.Slice{
				Name:   "worker",
				Kind:   plan.SliceKindWorker,
				Image:  "myorg/worker:1.0",
				Probes: []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready"}},
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				assert.Nil(t, c.ReadinessProbe, "a probe on a guessed port never passes, so none should be emitted")
			},
		},
		{
			name: "tcp probe keeps its own port",
			slice: plan.Slice{
				Name:   "db",
				Kind:   plan.SliceKindDatabase,
				Image:  "postgres:16",
				Port:   5432,
				Probes: []plan.ProbeSpec{{Type: "liveness", TCPPort: 5433}},
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				require.NotNil(t, c.LivenessProbe)
				require.NotNil(t, c.LivenessProbe.TCPSocket)
				assert.Nil(t, c.LivenessProbe.HTTPGet)
				assert.Equal(t, int32(5433), c.LivenessProbe.TCPSocket.Port.IntVal)
			},
		},
		{
			name: "a check with nothing to check emits no probe",
			slice: plan.Slice{
				Name:   "web",
				Kind:   plan.SliceKindWeb,
				Image:  "myorg/api:1.0",
				Port:   8080,
				Probes: []plan.ProbeSpec{{Type: "readiness"}},
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				assert.Nil(t, c.ReadinessProbe, "an empty probe handler is rejected by the API server")
			},
		},
		{
			name: "legacy health path resolves a named port too",
			slice: plan.Slice{
				Name:       "web",
				Kind:       plan.SliceKindWeb,
				Image:      "myorg/api:1.0",
				Ports:      []plan.PortSpec{{Name: "http", Port: 8065}},
				HealthPath: "/healthz",
			},
			assertF: func(t *testing.T, c corev1.Container) {
				t.Helper()

				require.NotNil(t, c.ReadinessProbe)
				require.NotNil(t, c.ReadinessProbe.HTTPGet)
				assert.Equal(t, int32(8065), c.ReadinessProbe.HTTPGet.Port.IntVal)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dep := k8s.BuildDeployment(tt.slice, "default", "probeports")
			require.Len(t, dep.Spec.Template.Spec.Containers, 1)

			tt.assertF(t, dep.Spec.Template.Spec.Containers[0])
		})
	}
}
