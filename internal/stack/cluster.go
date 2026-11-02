package stack

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// provider drives one local cluster tool.
type provider interface {
	// binary is the CLI the provider needs on PATH.
	binary() string
	exists(ctx context.Context) (bool, error)
	create(ctx context.Context) error
	delete(ctx context.Context) error
	// kubeconfig writes the cluster's kubeconfig to path, without touching
	// the user's default kubeconfig.
	kubeconfig(ctx context.Context, path string) error
	// importArchive loads a `docker save` archive into every node.
	importArchive(ctx context.Context, tarPath string) error
	// nodes lists the cluster's node containers.
	nodes(ctx context.Context) ([]node, error)
	// network is the docker network the nodes share, and server the API
	// address a container on that network reaches.
	network() string
	server() string
}

// node is one cluster node container.
type node struct {
	Name         string
	ControlPlane bool
}

func newProvider(e *Engine) (provider, error) {
	switch e.stack.Cluster.Provider {
	case ProviderKind:
		return &kindProvider{e: e}, nil
	case ProviderK3d:
		return &k3dProvider{e: e}, nil
	default:
		return nil, fmt.Errorf("unknown cluster provider %q", e.stack.Cluster.Provider)
	}
}

type kindProvider struct{ e *Engine }

func (p *kindProvider) binary() string { return "kind" }

func (p *kindProvider) exists(ctx context.Context) (bool, error) {
	out, err := p.e.query(ctx, Cmd{Name: "kind", Args: []string{"get", "clusters"}})
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(line) == p.e.stack.ClusterName() {
			return true, nil
		}
	}
	return false, nil
}

// kindConfig renders the cluster config. The API server address is pinned to
// loopback explicitly — it is kind's default, but this file is the contract
// that nothing in a stack listens beyond this machine, so it says so.
func (p *kindProvider) kindConfig() ([]byte, error) {
	cl := p.e.stack.Cluster

	networking := map[string]any{"apiServerAddress": "127.0.0.1"}
	if cl.DisableDefaultCNI {
		networking["disableDefaultCNI"] = true
	}
	if cl.PodSubnet != "" {
		networking["podSubnet"] = cl.PodSubnet
	}
	if cl.ServiceSubnet != "" {
		networking["serviceSubnet"] = cl.ServiceSubnet
	}

	control := map[string]any{"role": "control-plane"}
	if len(cl.Ports) > 0 {
		var mappings []map[string]any
		for _, pm := range cl.Ports {
			mappings = append(mappings, map[string]any{
				"containerPort": pm.Node,
				"hostPort":      pm.Host,
				"listenAddress": "127.0.0.1",
			})
		}
		control["extraPortMappings"] = mappings
	}
	nodes := []map[string]any{control}
	for range cl.Workers {
		nodes = append(nodes, map[string]any{"role": "worker"})
	}

	cfg := map[string]any{
		"kind":       "Cluster",
		"apiVersion": "kind.x-k8s.io/v1alpha4",
		"name":       p.e.stack.ClusterName(),
		"networking": networking,
		"nodes":      nodes,
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("kind config: %w", err)
	}
	return b, nil
}

func (p *kindProvider) create(ctx context.Context) error {
	cfg, err := p.kindConfig()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(p.e.dir, "kind-config.yaml")
	if err := p.e.writeFile(cfgPath, cfg); err != nil {
		return err
	}
	args := []string{"create", "cluster", "--name", p.e.stack.ClusterName(), "--config", cfgPath,
		"--kubeconfig", p.e.kubeconfigPath()}
	if img := p.e.stack.Cluster.NodeImage; img != "" {
		args = append(args, "--image", img)
	}
	return p.e.run(ctx, Cmd{Name: "kind", Args: args})
}

func (p *kindProvider) delete(ctx context.Context) error {
	return p.e.run(ctx, Cmd{Name: "kind", Args: []string{"delete", "cluster", "--name", p.e.stack.ClusterName(),
		"--kubeconfig", p.e.kubeconfigPath()}})
}

func (p *kindProvider) kubeconfig(ctx context.Context, path string) error {
	return p.e.run(ctx, Cmd{Name: "kind", Args: []string{"export", "kubeconfig", "--name", p.e.stack.ClusterName(),
		"--kubeconfig", path}})
}

func (p *kindProvider) importArchive(ctx context.Context, tarPath string) error {
	return p.e.run(ctx, Cmd{Name: "kind", Args: []string{"load", "image-archive", tarPath, "--name", p.e.stack.ClusterName()}})
}

func (p *kindProvider) nodes(ctx context.Context) ([]node, error) {
	out, err := p.e.query(ctx, Cmd{Name: "kind", Args: []string{"get", "nodes", "--name", p.e.stack.ClusterName()}})
	if err != nil {
		return nil, err
	}
	var nodes []node
	for name := range strings.FieldsSeq(string(out)) {
		nodes = append(nodes, node{Name: name, ControlPlane: strings.Contains(name, "-control-plane")})
	}
	return nodes, nil
}

func (p *kindProvider) network() string { return "kind" }

func (p *kindProvider) server() string {
	return "https://" + p.e.stack.ClusterName() + "-control-plane:6443"
}

type k3dProvider struct{ e *Engine }

func (p *k3dProvider) nodes(ctx context.Context) ([]node, error) {
	out, err := p.e.query(ctx, Cmd{Name: "docker", Args: []string{"ps",
		"--filter", "label=k3d.cluster=" + p.e.stack.ClusterName(),
		"--format", `{{.Names}} {{.Label "k3d.role"}}`}})
	if err != nil {
		return nil, err
	}
	var nodes []node
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || (f[1] != "server" && f[1] != "agent") {
			continue // the load balancer and tools containers are not nodes
		}
		nodes = append(nodes, node{Name: f[0], ControlPlane: f[1] == "server"})
	}
	return nodes, nil
}

func (p *k3dProvider) network() string { return "k3d-" + p.e.stack.ClusterName() }

func (p *k3dProvider) server() string {
	return "https://k3d-" + p.e.stack.ClusterName() + "-server-0:6443"
}

// k3d edits whatever kubeconfig $KUBECONFIG names when it creates or deletes
// a cluster, flags or not. Pointing it at the stack's own file keeps the
// user's kubeconfig — and any read-only one $KUBECONFIG names — untouched.
func (p *k3dProvider) cmd(args ...string) Cmd {
	return Cmd{Name: "k3d", Args: args, Env: []string{"KUBECONFIG=" + p.e.kubeconfigPath()}}
}

func (p *k3dProvider) binary() string { return "k3d" }

func (p *k3dProvider) exists(ctx context.Context) (bool, error) {
	out, err := p.e.query(ctx, p.cmd("cluster", "list", "--no-headers"))
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 && f[0] == p.e.stack.ClusterName() {
			return true, nil
		}
	}
	return false, nil
}

func (p *k3dProvider) create(ctx context.Context) error {
	cl := p.e.stack.Cluster

	// k3d binds the API server to every interface unless told otherwise, so
	// the port is chosen here and pinned to loopback.
	apiPort, err := p.e.freePort()
	if err != nil {
		return err
	}

	args := []string{"cluster", "create", p.e.stack.ClusterName(),
		"--api-port", "127.0.0.1:" + strconv.Itoa(apiPort),
		"--agents", strconv.Itoa(cl.Workers),
		"--kubeconfig-update-default=false",
		"--kubeconfig-switch-context=false",
		"--timeout", "300s",
	}
	if cl.NodeImage != "" {
		args = append(args, "--image", cl.NodeImage)
	}
	if cl.DisableDefaultCNI {
		args = append(args,
			"--k3s-arg", "--flannel-backend=none@server:*",
			"--k3s-arg", "--disable-network-policy@server:*")
	}
	if cl.PodSubnet != "" {
		args = append(args, "--k3s-arg", "--cluster-cidr="+cl.PodSubnet+"@server:*")
	}
	if cl.ServiceSubnet != "" {
		dns, err := clusterDNS(cl.ServiceSubnet)
		if err != nil {
			return err
		}
		args = append(args,
			"--k3s-arg", "--service-cidr="+cl.ServiceSubnet+"@server:*",
			"--k3s-arg", "--cluster-dns="+dns+"@server:*")
	}
	for _, pm := range cl.Ports {
		args = append(args, "--port", fmt.Sprintf("127.0.0.1:%d:%d@server:0", pm.Host, pm.Node))
	}
	if err := p.e.run(ctx, p.cmd(args...)); err != nil {
		return fmt.Errorf("%w\n\nIf k3d reported \"stopped returning log lines\", this k3d cannot read container logs from "+
			"this Docker Engine (k3d v5.6.3 against Docker 29 does this; v5.9.0 works). Upgrade k3d, or run the stack on kind "+
			"with --provider kind", err)
	}
	return nil
}

func (p *k3dProvider) delete(ctx context.Context) error {
	return p.e.run(ctx, p.cmd("cluster", "delete", p.e.stack.ClusterName()))
}

func (p *k3dProvider) kubeconfig(ctx context.Context, path string) error {
	out, err := p.e.query(ctx, p.cmd("kubeconfig", "get", p.e.stack.ClusterName()))
	if err != nil {
		return err
	}
	return p.e.writeFile(path, out)
}

func (p *k3dProvider) importArchive(ctx context.Context, tarPath string) error {
	return p.e.run(ctx, p.cmd("image", "import", tarPath, "-c", p.e.stack.ClusterName()))
}

// assertLoopback refuses a kubeconfig that points anywhere but this machine.
// It is the backstop for "stacks are local": whatever a provider or a stack
// file does, no command runs against a server that is not loopback.
func assertLoopback(kubeconfig []byte) error {
	var kc struct {
		Clusters []struct {
			Name    string `yaml:"name"`
			Cluster struct {
				Server string `yaml:"server"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
	}
	if err := yaml.Unmarshal(kubeconfig, &kc); err != nil {
		return fmt.Errorf("read kubeconfig: %w", err)
	}
	if len(kc.Clusters) == 0 {
		return fmt.Errorf("kubeconfig names no cluster")
	}
	for _, c := range kc.Clusters {
		u, err := url.Parse(c.Cluster.Server)
		if err != nil {
			return fmt.Errorf("cluster %s: server %q: %w", c.Name, c.Cluster.Server, err)
		}
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return fmt.Errorf("cluster %s serves its API at %s, which is not this machine; stacks only run on local clusters",
				c.Name, c.Cluster.Server)
		}
	}
	return nil
}

// clusterDNS is the conventional CoreDNS address in a Service range: .10.
func clusterDNS(subnet string) (string, error) {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("cluster.serviceSubnet %q: %w", subnet, err)
	}
	ip := ipnet.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("cluster.serviceSubnet %q must be IPv4", subnet)
	}
	dns := net.IPv4(ip[0], ip[1], ip[2], ip[3]+10)
	return dns.String(), nil
}

func freeLoopbackPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("pick a free port: %w", err)
	}
	defer func() { _ = l.Close() }()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("pick a free port: unexpected address %v", l.Addr())
	}
	return addr.Port, nil
}

func readFileIfExists(path string) ([]byte, bool) {
	b, err := os.ReadFile(path) //nolint:gosec // engine-owned state path
	if err != nil {
		return nil, false
	}
	return b, true
}
