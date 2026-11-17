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
