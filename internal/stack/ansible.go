package stack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// runnerDockerfile is the image playbooks run in. Ansible runs in a container
// rather than on the host so a stack needs nothing installed beyond docker,
// every machine runs the same pinned versions, and `localhost` in a playbook
// is a throwaway container instead of the user's own machine.
//
// community.docker reaches node containers through the Docker API (the
// docker_api connection, hence the docker SDK), and kubernetes.core reaches
// the cluster API (hence the kubernetes client). ansible.posix and
// community.general are there because most real playbooks use them.
const runnerDockerfile = `FROM python:3.13-alpine3.22
RUN apk add --no-cache openssh-client git \
 && pip install --no-cache-dir ansible-core==2.21.5 kubernetes==37.0.1 docker==7.2.0 \
 && ansible-galaxy collection install --collections-path /usr/share/ansible/collections \
      community.docker:==5.4.0 kubernetes.core:==6.6.0 ansible.posix:==2.2.2 community.general:==13.5.0
`

// runnerImage names the runner image after its Dockerfile, so changing the
// pinned versions builds a new image instead of reusing a stale one.
func runnerImage() string {
	sum := sha256.Sum256([]byte(runnerDockerfile))
	return "mozza/ansible-runner:" + hex.EncodeToString(sum[:])[:12]
}

// Paths inside the runner container.
const (
	runnerSource = "/src"
	runnerWork   = "/work"
)

func (e *Engine) ensureRunner(ctx context.Context) error {
	ref := runnerImage()
	if e.imagePresent(ctx, ref) {
		return nil
	}
	e.info("building the Ansible runner image %s (once)", ref)
	return e.run(ctx, Cmd{Name: "docker", Args: []string{"build", "-t", ref, "-"},
		Stdin: strings.NewReader(runnerDockerfile)})
}

// runAnsible runs one component's playbook in the runner container.
func (e *Engine) runAnsible(ctx context.Context, c Component) error {
	a := c.Ansible
	p, err := newProvider(e)
	if err != nil {
		return err
	}
	if err = e.ensureRunner(ctx); err != nil {
		return err
	}

	e.ansibleEnv = a.Env
	work := filepath.Join(e.dir, "ansible", c.Name)
	if err = e.mkdirAll(work); err != nil {
		return err
	}

	nodes, err := p.nodes(ctx)
	if err != nil {
		return fmt.Errorf("list cluster nodes: %w", err)
	}
	if err = e.checkNodePython(ctx, a, nodes); err != nil {
		return err
	}
	inventory, err := ansibleInventory(nodes, a.Groups)
	if err != nil {
		return err
	}
	if err = e.writeFile(filepath.Join(work, "inventory.yml"), inventory); err != nil {
		return err
	}

	vars, err := e.expandSecretRefs(a.ExtraVars)
	if err != nil {
		return err
	}
	varsYAML, err := yaml.Marshal(vars)
	if err != nil {
		return fmt.Errorf("render extraVars: %w", err)
	}
	if err = e.writeFile(filepath.Join(work, "extravars.yml"), varsYAML); err != nil {
		return err
	}

	kubeconfig, err := e.internalKubeconfig(p)
	if err != nil {
		return err
	}
	if err = e.writeFile(filepath.Join(work, "kubeconfig"), kubeconfig); err != nil {
		return err
	}

	if a.Requirements != "" {
		e.info("installing %s", e.inSource(a.Requirements))
		if err = e.run(ctx, e.runnerCmd(p, work, a.Dir, "sh", "-c",
			"ansible-galaxy collection install -r "+shellQuote(e.inSource(a.Requirements))+
				" -p "+runnerWork+"/collections && "+
				"ansible-galaxy role install -r "+shellQuote(e.inSource(a.Requirements))+
				" -p "+runnerWork+"/roles")); err != nil {
			return fmt.Errorf("install %s: %w", a.Requirements, err)
		}
	}

	args := []string{"ansible-playbook", "-i", runnerWork + "/inventory.yml", "-e", "@" + runnerWork + "/extravars.yml"}
	if len(a.Tags) > 0 {
		args = append(args, "--tags", strings.Join(a.Tags, ","))
	}
	if len(a.SkipTags) > 0 {
		args = append(args, "--skip-tags", strings.Join(a.SkipTags, ","))
	}
	if a.Limit != "" {
		args = append(args, "--limit", a.Limit)
	}
	args = append(args, e.inSource(a.Playbook))

	e.info("ansible-playbook %s", relTo(e.stack.SourceDir, a.Playbook))
	return e.run(ctx, e.runnerCmd(p, work, a.Dir, args...))
}

// runnerCmd is `docker run` for the runner: the source tree read-only, the
// component's work directory read-write, on the cluster's own network.
//
// The Docker socket is mounted because node targets are reached through the
// Docker API; it is the same daemon the cluster runs on. The inventory, not
// the network, is what limits a playbook to local targets.
func (e *Engine) runnerCmd(p provider, work, dir string, args ...string) Cmd {
	run := []string{"run", "--rm",
		"--network", p.network(),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", e.stack.SourceDir + ":" + runnerSource + ":ro",
		"-v", work + ":" + runnerWork,
		"-w", e.inSource(dir),
		"-e", "ANSIBLE_COLLECTIONS_PATH=" + runnerWork + "/collections:/usr/share/ansible/collections",
		"-e", "ANSIBLE_ROLES_PATH=" + runnerWork + "/roles:" + e.inSource(dir) + "/roles",
		"-e", "ANSIBLE_HOST_KEY_CHECKING=False",
		"-e", "ANSIBLE_RETRY_FILES_ENABLED=False",
		"-e", "ANSIBLE_FORCE_COLOR=1",
		// There is no terminal to answer a prompt; a tree whose ansible.cfg
		// sets ask_vault_pass would otherwise stop dead.
		"-e", "ANSIBLE_ASK_VAULT_PASS=False",
		"-e", "KUBECONFIG=" + runnerWork + "/kubeconfig",
		"-e", "K8S_AUTH_KUBECONFIG=" + runnerWork + "/kubeconfig",
	}
	for _, k := range sortedKeys(e.ansibleEnv) {
		run = append(run, "-e", k+"="+e.ansibleEnv[k])
	}
	run = append(run, runnerImage())
	return Cmd{Name: "docker", Args: append(run, args...)}
}

// inSource maps a path in the source tree to where the runner sees it.
func (e *Engine) inSource(path string) string {
	return filepath.ToSlash(filepath.Join(runnerSource, relTo(e.stack.SourceDir, path)))
}

func relTo(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return rel
}

// internalKubeconfig is the stack's kubeconfig with the API server rewritten
// to the address a container on the cluster network reaches. The loopback
// address in the host kubeconfig means the runner container itself there.
func (e *Engine) internalKubeconfig(p provider) ([]byte, error) {
	if e.DryRun {
		return nil, nil
	}
	b, ok := readFileIfExists(e.kubeconfigPath())
	if !ok {
		return nil, fmt.Errorf("no kubeconfig at %s", e.kubeconfigPath())
	}
	var kc map[string]any
	if err := yaml.Unmarshal(b, &kc); err != nil {
		return nil, fmt.Errorf("read kubeconfig: %w", err)
	}
	clusters, _ := kc["clusters"].([]any)
	for _, c := range clusters {
		entry, _ := c.(map[string]any)
		if inner, ok := entry["cluster"].(map[string]any); ok {
			inner["server"] = p.server()
		}
	}
	out, err := yaml.Marshal(kc)
	if err != nil {
		return nil, fmt.Errorf("write kubeconfig: %w", err)
	}
	return out, nil
}

// ansibleInventory renders the inventory: one group per local target, plus
// the playbook's own group names as children of the targets they map to.
func ansibleInventory(nodes []node, aliases map[string]string) ([]byte, error) {
	hosts := func(want func(node) bool) map[string]any {
		out := map[string]any{}
		for _, n := range nodes {
			if want(n) {
				out[n.Name] = map[string]any{"ansible_host": n.Name}
			}
		}
		return out
	}
	controlPlane := hosts(func(n node) bool { return n.ControlPlane })
	workers := hosts(func(n node) bool { return !n.ControlPlane })

	children := map[string]any{
		"nodes": map[string]any{
			"children": map[string]any{
				"control_plane": map[string]any{"hosts": controlPlane},
				"workers":       map[string]any{"hosts": workers},
			},
			"vars": map[string]any{
				"ansible_connection":         "community.docker.docker_api",
				"ansible_python_interpreter": "/usr/bin/python3",
				// Node containers run as root and have no sudo. A playbook
				// written for VMs says `become: true`; as a connection
				// variable this outranks that play keyword, so it runs as is.
				"ansible_become": false,
				// A tree's ansible.cfg often names the VM login user
				// (remote_user); the docker connection would exec as it.
				"ansible_user": "root",
			},
		},
		"runner": map[string]any{
			"hosts": map[string]any{"localhost": map[string]any{
				"ansible_connection":         "local",
				"ansible_python_interpreter": "/usr/local/bin/python3",
				"ansible_become":             false,
			}},
		},
	}

	names := make([]string, 0, len(aliases))
	for g := range aliases {
		names = append(names, g)
	}
	sort.Strings(names)
	for _, group := range names {
		if _, taken := children[group]; taken || group == "control_plane" || group == "workers" || group == "all" {
			return nil, fmt.Errorf("ansible.groups: %q is already a target name", group)
		}
		children[group] = map[string]any{"children": map[string]any{aliases[group]: map[string]any{}}}
	}

	b, err := yaml.Marshal(map[string]any{"all": map[string]any{"children": children}})
	if err != nil {
		return nil, fmt.Errorf("render inventory: %w", err)
	}
	return b, nil
}

// checkNodePython fails early when a playbook targets nodes that cannot run
// Ansible modules. kind nodes are Debian with python3; k3d's k3s nodes have
// no Python at all, and the module failures that would follow say nothing
// about why.
func (e *Engine) checkNodePython(ctx context.Context, a *Ansible, nodes []node) error {
	if e.DryRun || !targetsNodes(a) {
		return nil
	}
	for _, n := range nodes {
		if _, err := e.runner.Output(ctx, Cmd{Name: "docker", Args: []string{"exec", n.Name, "python3", "--version"}}); err != nil {
			return fmt.Errorf("node %s has no python3, so Ansible cannot run tasks on it; run this stack on kind "+
				"(--provider kind), or map the playbook's groups to runner", n.Name)
		}
	}
	return nil
}

// targetsNodes reports whether any group could reach a node: an explicit
// mapping to a node target, or no mapping at all (a playbook on "all").
func targetsNodes(a *Ansible) bool {
	if len(a.Groups) == 0 {
		return true
	}
	for _, t := range a.Groups {
		if t != "runner" {
			return true
		}
	}
	return false
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
