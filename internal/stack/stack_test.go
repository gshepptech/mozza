package stack

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// fakeRunner records every command and answers queries from a script.
type fakeRunner struct {
	runs    []Cmd
	stdins  []string
	answers map[string]string // command-line prefix -> stdout
	fail    map[string]bool   // command-line prefix -> error
}

func (f *fakeRunner) match(c Cmd, m map[string]string) (string, bool) {
	line := c.String()
	for prefix, out := range m {
		if strings.HasPrefix(line, prefix) {
			return out, true
		}
	}
	return "", false
}

func (f *fakeRunner) Run(_ context.Context, c Cmd, _ io.Writer) error {
	f.runs = append(f.runs, c)
	stdin := ""
	if c.Stdin != nil {
		b, _ := io.ReadAll(c.Stdin)
		stdin = string(b)
	}
	f.stdins = append(f.stdins, stdin)
	for prefix := range f.fail {
		if strings.HasPrefix(c.String(), prefix) {
			return errors.New("scripted failure")
		}
	}
	return nil
}

func (f *fakeRunner) Output(_ context.Context, c Cmd) ([]byte, error) {
	f.runs = append(f.runs, c)
	f.stdins = append(f.stdins, "")
	if out, ok := f.match(c, f.answers); ok {
		return []byte(out), nil
	}
	return nil, errors.New("not scripted")
}

func (f *fakeRunner) lines() []string {
	out := make([]string, len(f.runs))
	for i, c := range f.runs {
		out[i] = c.String()
	}
	return out
}

func indexOf(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

const minimalStack = `
name: demo
cluster:
  provider: kind
components:
  - name: app
    namespace: demo
    helm:
      chart: oci://example.test/charts/app
`

func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := Parse([]byte(minimalStack + "\nclustr: {}\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clustr")
}

func TestValidateReportsEveryProblem(t *testing.T) {
	_, err := Parse([]byte(`
name: Bad_Name
cluster: {provider: minikube}
secrets:
  - name: s
    namespace: n
    data:
      k: {generate: hex, value: also}
components:
  - name: x
    manifests:
      - url: http://insecure.test/x.yaml
`))
	require.Error(t, err)
	for _, want := range []string{"Bad_Name", "minikube", "exactly one of generate", "url must be https"} {
		assert.Contains(t, err.Error(), want)
	}
}

func TestUseTakesOnlyVersionAndValues(t *testing.T) {
	_, err := Parse([]byte(`
name: demo
cluster: {provider: kind}
components:
  - use: cnpg
    namespace: elsewhere
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "takes only version and values")
}

func TestBuiltinCatalogParses(t *testing.T) {
	cat := Catalog{}
	stacks, err := cat.Stacks()
	require.NoError(t, err)
	require.NotEmpty(t, stacks)
	for _, e := range stacks {
		s, lerr := cat.Load(e.Name)
		require.NoError(t, lerr, e.Name)
		for _, c := range s.Components {
			if c.Use != "" {
				_, aerr := cat.expandAddon(c)
				require.NoError(t, aerr, "%s uses %s", e.Name, c.Use)
			}
		}
	}
	addons, err := cat.Addons()
	require.NoError(t, err)
	for _, e := range addons {
		_, err := cat.addon(e.Name)
		require.NoError(t, err, e.Name)
	}
}

func TestResolveInlinesAddonAndFoldsNeeds(t *testing.T) {
	src := t.TempDir()
	s, err := Parse([]byte(`
name: demo
source: {path: ` + src + `}
cluster: {provider: kind}
components:
  - use: calico
    version: v9.9.9
  - use: cnpg
    values: {replicaCount: 2}
  - name: app
    namespace: demo
    helm:
      chart: charts/app
      valuesFiles: [charts/app/values-local.yaml]
    values:
      image: {repository: app, tag: "${stack}"}
`))
	require.NoError(t, err)

	r, err := Catalog{}.Resolve(s, ResolveOptions{})
	require.NoError(t, err)

	assert.True(t, r.Cluster.DisableDefaultCNI, "calico needs the default CNI off")
	assert.Equal(t, "192.168.0.0/16", r.Cluster.PodSubnet)
	assert.Contains(t, r.Components[0].Manifests[0].URL, "/v9.9.9/", "the stack's version pin reaches the addon's URL")
	assert.Equal(t, "0.29.1", r.Components[1].Helm.Version)
	assert.Equal(t, 2, r.Components[1].Values["replicaCount"])
	assert.Equal(t, filepath.Join(src, "charts/app"), r.Components[2].Helm.Chart)
	assert.Equal(t, filepath.Join(src, "charts/app/values-local.yaml"), r.Components[2].Helm.ValuesFiles[0])
	assert.Equal(t, "demo", r.Components[2].Values["image"].(map[string]any)["tag"])

	// Resolve must not reach back into the loaded stack.
	assert.Equal(t, "charts/app", s.Components[2].Helm.Chart)
}

func TestResolveSourceOverrides(t *testing.T) {
	flagDir, envDir := t.TempDir(), t.TempDir()
	s, err := Parse([]byte(minimalStack + "source: {path: /does/not/exist}\n"))
	require.NoError(t, err)

	_, err = Catalog{}.Resolve(s, ResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MOZZA_DEMO_SOURCE")

	t.Setenv("MOZZA_DEMO_SOURCE", envDir)
	r, err := Catalog{}.Resolve(s, ResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, envDir, r.SourceDir)

	r, err = Catalog{}.Resolve(s, ResolveOptions{Source: flagDir})
	require.NoError(t, err)
	assert.Equal(t, flagDir, r.SourceDir)

	r, err = Catalog{}.Resolve(s, ResolveOptions{NoSource: true})
	require.NoError(t, err)
	assert.Empty(t, r.SourceDir)
}

func TestResolveRejectsUnknownVariable(t *testing.T) {
	s, err := Parse([]byte(minimalStack + "    values: {x: \"${sorce}\"}\n"))
	require.NoError(t, err)
	_, err = Catalog{}.Resolve(s, ResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "${sorce}")
}

func TestUserCatalogShadowsBuiltin(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, "addons", "cnpg.yaml"), []byte(`
name: cnpg
description: patched
version: 1.0.0
namespace: cnpg-system
helm: {repo: https://example.test, chart: cnpg, version: "${version}"}
`), 0o644))

	cat := Catalog{UserDir: home}
	addons, err := cat.Addons()
	require.NoError(t, err)
	for _, a := range addons {
		if a.Name == "cnpg" {
			assert.Equal(t, "patched", a.Description)
		}
	}
	c, err := cat.expandAddon(Component{Use: "cnpg"})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", c.Helm.Version)
}

func TestSecretStoreReusesGeneratedValues(t *testing.T) {
	dir := t.TempDir()
	sec := Secret{Name: "s", Namespace: "n", Data: map[string]Value{
		"pw":  {Generate: "password", Show: true},
		"key": {Generate: "age-key"},
		"hex": {Generate: "hex", Length: 16},
		"lit": {Value: "admin"},
	}}

	st, err := openSecretStore(dir)
	require.NoError(t, err)
	first, err := st.resolve(sec)
	require.NoError(t, err)
	require.NoError(t, st.save())

	assert.Len(t, first["pw"], 24)
	assert.True(t, strings.HasPrefix(first["key"], "AGE-SECRET-KEY-1"))
	assert.Len(t, first["hex"], 32)
	assert.Equal(t, "admin", first["lit"])

	info, err := os.Stat(filepath.Join(dir, "secrets.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	st2, err := openSecretStore(dir)
	require.NoError(t, err)
	second, err := st2.resolve(sec)
	require.NoError(t, err)
	assert.Equal(t, first, second, "a second up must not rotate credentials")

	shown := st2.shown([]Secret{sec})
	require.Len(t, shown, 1)
	assert.Equal(t, "n/s/pw", shown[0].Ref)
}

func TestAssertLoopback(t *testing.T) {
	kc := func(server string) []byte {
		return []byte("clusters:\n- name: c\n  cluster:\n    server: " + server + "\n")
	}
	assert.NoError(t, assertLoopback(kc("https://127.0.0.1:6443")))
	assert.NoError(t, assertLoopback(kc("https://localhost:6443")))
	assert.NoError(t, assertLoopback(kc("https://[::1]:6443")))
	require.Error(t, assertLoopback(kc("https://10.0.0.5:6443")))
	require.Error(t, assertLoopback(kc("https://api.example.com")))
	require.Error(t, assertLoopback(kc("https://0.0.0.0:6443")))
}

func TestKindConfigBindsLoopback(t *testing.T) {
	e := &Engine{stack: &Resolved{Stack: &Stack{Name: "demo", Cluster: Cluster{
		Provider: ProviderKind, Workers: 1, DisableDefaultCNI: true, PodSubnet: "10.9.0.0/16",
		Ports: []PortMapping{{Host: 18080, Node: 30080}},
	}}}}
	b, err := (&kindProvider{e: e}).kindConfig()
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(b, &cfg))
	net := cfg["networking"].(map[string]any)
	assert.Equal(t, "127.0.0.1", net["apiServerAddress"])
	assert.Equal(t, true, net["disableDefaultCNI"])
	assert.Equal(t, "10.9.0.0/16", net["podSubnet"])
	nodes := cfg["nodes"].([]any)
	require.Len(t, nodes, 2)
	mapping := nodes[0].(map[string]any)["extraPortMappings"].([]any)[0].(map[string]any)
	assert.Equal(t, "127.0.0.1", mapping["listenAddress"])
}

// engineFor builds an engine over a fake runner with a kubeconfig already in
// place, the way a provider leaves it.
func engineFor(t *testing.T, doc string, fr *fakeRunner) *Engine {
	t.Helper()
	s, err := Parse([]byte(doc))
	require.NoError(t, err)
	r, err := Catalog{}.Resolve(s, ResolveOptions{NoSource: true})
	require.NoError(t, err)
	e := NewEngine(r, fr, io.Discard, t.TempDir())
	e.freePort = func() (int, error) { return 41000, nil }
	require.NoError(t, os.MkdirAll(e.dir, 0o700))
	require.NoError(t, os.WriteFile(e.kubeconfigPath(),
		[]byte("clusters:\n- name: x\n  cluster:\n    server: https://127.0.0.1:41000\n"), 0o600))
	return e
}

func TestUpRunsStepsInOrderAndKeepsSecretsOffArgv(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("needs a POSIX host for PATH lookups")
	}
	fr := &fakeRunner{answers: map[string]string{
		"docker info":    "29.0.0",
		"docker version": "linux/arm64",
		"kind get":       "other\n",
		"kubectl":        "ok",
	}}
	e := engineFor(t, `
name: demo
cluster: {provider: kind}
images:
  - ref: busybox:1
secrets:
  - name: creds
    namespace: demo
    data:
      password: {generate: password}
components:
  - name: app
    namespace: demo
    helm: {chart: oci://example.test/app, version: 1.2.3}
    values:
      pw: ${secret:demo/creds/password}
`, fr)
	t.Setenv("PATH", fakeTools(t))

	require.NoError(t, e.Up(context.Background()))

	lines := fr.lines()
	order := []string{
		"docker pull busybox:1",
		"kind create cluster --name mozza-demo",
		"docker save --platform linux/arm64",
		"kind load image-archive",
		"helm upgrade --install app oci://example.test/app",
	}
	last := -1
	for _, want := range order {
		i := indexOf(lines, want)
		require.GreaterOrEqual(t, i, 0, "missing %q in\n%s", want, strings.Join(lines, "\n"))
		assert.Greater(t, i, last, "%q out of order", want)
		last = i
	}

	st, err := openSecretStore(e.dir)
	require.NoError(t, err)
	pw := st.values["demo/creds/password"]
	require.NotEmpty(t, pw)
	for _, l := range lines {
		assert.NotContains(t, l, pw, "a secret value reached a command line")
	}
	joined := strings.Join(fr.stdins, "\n")
	assert.Contains(t, joined, "kind: Secret")
	assert.Contains(t, joined, pw)

	values, err := os.ReadFile(filepath.Join(e.dir, "values", "app.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(values), pw, "${secret:...} resolves into the rendered values file")

	recorded, ok := e.loadState()
	require.True(t, ok)
	assert.True(t, recorded.CreatedCluster)
}

func TestUpReusesExistingClusterAndDownRefusesIt(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{
		"docker info": "29.0.0",
		"kind get":    "mozza-demo\n",
		"kubectl":     "ok",
	}}
	e := engineFor(t, minimalStack, fr)
	t.Setenv("PATH", fakeTools(t))

	require.NoError(t, e.Up(context.Background()))
	assert.Equal(t, -1, indexOf(fr.lines(), "kind create"), "an existing cluster is reused")

	err := e.Down(context.Background(), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not created by")
	assert.Equal(t, -1, indexOf(fr.lines(), "kind delete"))
}

func TestStageChartCopiesChartsWithDependencies(t *testing.T) {
	src := t.TempDir()
	chart := filepath.Join(src, "chart")
	require.NoError(t, os.MkdirAll(chart, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chart, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: c\nversion: 0.1.0\ndependencies:\n- name: d\n  version: 1.0.0\n  repository: https://example.test\n"), 0o644))

	fr := &fakeRunner{}
	e := engineFor(t, minimalStack, fr)
	staged, err := e.stageChart(context.Background(), "app", chart)
	require.NoError(t, err)

	assert.NotEqual(t, chart, staged)
	assert.FileExists(t, filepath.Join(staged, "Chart.yaml"))
	assert.Equal(t, 0, indexOf(fr.lines(), "helm dependency build "+staged))

	plain := filepath.Join(src, "plain")
	require.NoError(t, os.MkdirAll(plain, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plain, "Chart.yaml"), []byte("apiVersion: v2\nname: p\nversion: 0.1.0\n"), 0o644))
	got, err := e.stageChart(context.Background(), "plain", plain)
	require.NoError(t, err)
	assert.Equal(t, plain, got, "a chart without dependencies installs from the source tree")
}

func TestContextTarAppliesIgnoreAndExcludes(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	write(".dockerignore", "secret.txt\n")
	write("Dockerfile.app", "FROM scratch\n")
	write("main.go", "package main\n")
	write("secret.txt", "nope\n")
	write("web/node_modules/x/index.js", "x\n")
	write("web/src/app.ts", "app\n")
	write("app-binary", "elf\n")

	var buf bytes.Buffer
	require.NoError(t, writeContextTar(&buf, dir, []byte("FROM scratch\n"),
		[]string{"**/node_modules", "app-binary"}))

	names := map[string]bool{}
	tr := tar.NewReader(&buf)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		names[strings.TrimSuffix(h.Name, "/")] = true
	}
	for _, want := range []string{"main.go", "web/src/app.ts", "Dockerfile.app", streamedDockerfile} {
		assert.True(t, names[want], "missing %s", want)
	}
	for _, gone := range []string{"secret.txt", "web/node_modules", "web/node_modules/x/index.js", "app-binary", ".dockerignore"} {
		assert.False(t, names[gone], "%s should be excluded", gone)
	}
}

func TestWaitPollsForExistenceBeforeWaiting(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{"kubectl": "crd/x"}}
	e := engineFor(t, minimalStack, fr)
	require.NoError(t, e.wait(context.Background(), Wait{Resource: "crd/x", For: "condition=Established"}))
	lines := fr.lines()
	get, wait := indexOf(lines, "kubectl --kubeconfig"), -1
	for i, l := range lines {
		if strings.Contains(l, " wait crd/x --for condition=Established") {
			wait = i
		}
	}
	require.GreaterOrEqual(t, get, 0)
	assert.Greater(t, wait, get)
}

// fakeTools puts stub binaries for every tool the engine looks up on PATH.
func fakeTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, bin := range []string{"docker", "kind", "k3d", "kubectl", "helm"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, bin), []byte("#!/bin/sh\nexit 0\n"), 0o755)) //nolint:gosec // test stub
	}
	return dir
}

func TestRewriteDockerfileChangesOnlyTheStreamedCopy(t *testing.T) {
	e := &Engine{out: io.Discard}
	df := []byte("ARG VERSION=1.2.3-r0\nRUN apk add --no-cache \\\n    libssl3=3.5.8-r0 \\\n    git=2.54.0-r0 curl\n")
	got := e.rewriteDockerfile(df, []Rewrite{{
		Pattern: `(\s[a-z0-9][a-z0-9._+-]*)=[0-9][0-9A-Za-z._+-]*-r[0-9]+`,
		Replace: "$1",
	}})
	assert.Contains(t, string(got), "ARG VERSION=1.2.3-r0", "an ARG default is not a package pin")
	assert.Contains(t, string(got), "libssl3 \\")
	assert.Contains(t, string(got), "git curl")
}

func TestValidateRejectsBadRewritePattern(t *testing.T) {
	_, err := Parse([]byte(minimalStack + "images:\n  - ref: a:b\n    build:\n      context: .\n      rewrite: [{pattern: \"(\"}]\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular expression")
}

func TestSubnetsAreDistinctPerStackAndPinsWin(t *testing.T) {
	a := &Stack{Name: "inventory"}
	b := &Stack{Name: "storefront"}
	assignSubnets(a)
	assignSubnets(b)
	assert.NotEqual(t, a.Cluster.ServiceSubnet, b.Cluster.ServiceSubnet)
	assert.NotEqual(t, "10.43.0.0/16", a.Cluster.ServiceSubnet)

	pinned := &Stack{Name: "x", Cluster: Cluster{PodSubnet: "192.168.0.0/16"}}
	assignSubnets(pinned)
	assert.Equal(t, "192.168.0.0/16", pinned.Cluster.PodSubnet, "an addon's pod subnet is kept")

	dns, err := clusterDNS("10.170.0.0/16")
	require.NoError(t, err)
	assert.Equal(t, "10.170.0.10", dns)
}

func TestImageOverrideTagsInsteadOfBuilding(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{"docker image inspect": "sha256:abc"}}
	e := engineFor(t, minimalStack+"images:\n  - ref: app:local\n    build: {context: /tmp}\n", fr)
	e.ImageOverrides = map[string]string{"app:local": "registry.test/app:v1"}
	require.NoError(t, e.buildImages(context.Background()))
	lines := fr.lines()
	assert.GreaterOrEqual(t, indexOf(lines, "docker tag registry.test/app:v1 app:local"), 0)
	assert.Equal(t, -1, indexOf(lines, "docker build"))

	e.ImageOverrides = map[string]string{"typo:local": "x"}
	err := e.buildImages(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image typo:local")
}

func TestRestartForRollsOnlyWorkloadsUsingChangedImages(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{
		"kubectl": "Deployment web app app:local,sidecar:1,\nStatefulSet data db postgres:16,\nDeployment web init-only busybox:1,app:local,\n",
	}}
	e := engineFor(t, minimalStack, fr)
	require.NoError(t, e.restartFor(context.Background(), []string{"app:local"}))
	var restarts []string
	for _, l := range fr.lines() {
		if strings.Contains(l, "rollout restart") {
			restarts = append(restarts, l[strings.Index(l, "rollout"):])
		}
	}
	assert.Equal(t, []string{
		"rollout restart -n web deployment/app",
		"rollout restart -n web deployment/init-only",
	}, restarts)
}

func TestClearStuckRelease(t *testing.T) {
	for status, want := range map[string]string{
		"pending-install": "helm uninstall app",
		"pending-upgrade": "helm rollback app",
		"deployed":        "",
	} {
		fr := &fakeRunner{answers: map[string]string{"helm status": `{"info":{"status":"` + status + `"}}`}}
		e := engineFor(t, minimalStack, fr)
		require.NoError(t, e.clearStuckRelease(context.Background(), e.stack.Components[0]))
		if want == "" {
			assert.Equal(t, -1, indexOf(fr.lines(), "helm uninstall"), status)
			assert.Equal(t, -1, indexOf(fr.lines(), "helm rollback"), status)
			continue
		}
		assert.GreaterOrEqual(t, indexOf(fr.lines(), want), 0, status)
	}
}

func TestProviderOverrideAndProviderVariables(t *testing.T) {
	s, err := Parse([]byte(`
name: demo
cluster: {provider: kind}
components:
  - name: app
    namespace: demo
    helm: {chart: oci://example.test/app}
    values: {sc: "${storageClass}", origin: "${access.web.url}"}
access:
  - {name: web, namespace: demo, service: web, port: 80, localPort: 18000}
`))
	require.NoError(t, err)

	r, err := Catalog{}.Resolve(s, ResolveOptions{NoSource: true})
	require.NoError(t, err)
	assert.Equal(t, "standard", r.Components[0].Values["sc"])
	assert.Equal(t, "http://127.0.0.1:18000", r.Components[0].Values["origin"])

	r, err = Catalog{}.Resolve(s, ResolveOptions{NoSource: true, Provider: ProviderK3d})
	require.NoError(t, err)
	assert.Equal(t, "k3d-mozza-demo", r.KubeContext())
	assert.Equal(t, "local-path", r.Components[0].Values["sc"])

	_, err = Catalog{}.Resolve(s, ResolveOptions{NoSource: true, Provider: "minikube"})
	require.Error(t, err)
}

func TestLoadImagesSkipsUnchangedImagesOnExistingCluster(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{
		"docker version":       "linux/arm64",
		"docker image inspect": "sha256:same",
		"kubectl":              "uid-1",
	}}
	e := engineFor(t, minimalStack+"images:\n  - ref: app:local\n", fr)
	require.NoError(t, e.saveState(state{Images: map[string]string{"app:local": "sha256:same"}, ImagesCluster: "uid-1"}))

	changed, err := e.loadImages(context.Background(), &kindProvider{e: e}, false)
	require.NoError(t, err)
	assert.Empty(t, changed)
	assert.Equal(t, -1, indexOf(fr.lines(), "kind load"), "an unchanged image is not re-imported")

	_, err = e.loadImages(context.Background(), &kindProvider{e: e}, true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, indexOf(fr.lines(), "kind load"), 0, "a new cluster gets every image")
}

func TestUpReloadsImagesIntoARecreatedCluster(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{
		"docker info":          "29",
		"docker version":       "linux/arm64",
		"docker image inspect": "sha256:same",
		"kind get":             "mozza-demo\n",
		"kubectl":              "uid-NEW",
	}}
	e := engineFor(t, minimalStack+"images:\n  - ref: app:local\n", fr)
	t.Setenv("PATH", fakeTools(t))
	// IDs recorded against an earlier cluster of the same name.
	require.NoError(t, e.saveState(state{Images: map[string]string{"app:local": "sha256:same"}, ImagesCluster: "uid-OLD"}))

	require.NoError(t, e.Up(context.Background()))
	assert.GreaterOrEqual(t, indexOf(fr.lines(), "kind load image-archive"), 0)
}

func TestGitContextIsPassedToDockerUntouched(t *testing.T) {
	src := t.TempDir()
	s, err := Parse([]byte(minimalStack + "source: {path: " + src + "}\nimages:\n" +
		"  - ref: ui:mozza\n    build: {context: \"https://github.test/org/ui.git#v1\"}\n"))
	require.NoError(t, err)
	r, err := Catalog{}.Resolve(s, ResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, "https://github.test/org/ui.git#v1", r.Images[0].Build.Context)
	assert.Empty(t, r.Images[0].Build.Dockerfile)

	fr := &fakeRunner{}
	e := NewEngine(r, fr, io.Discard, t.TempDir())
	require.NoError(t, e.buildImage(context.Background(), r.Images[0]))
	assert.Equal(t, "docker build -t ui:mozza https://github.test/org/ui.git#v1", fr.lines()[0])

	_, err = Parse([]byte(minimalStack + "images:\n  - ref: ui:mozza\n    build: {context: \"https://github.test/ui.git\", exclude: [x]}\n"))
	require.Error(t, err)
}

func TestAnsibleInventoryNamesOnlyLocalTargets(t *testing.T) {
	b, err := ansibleInventory([]node{
		{Name: "mozza-x-control-plane", ControlPlane: true},
		{Name: "mozza-x-worker"},
	}, map[string]string{"k8s-servers": "control_plane", "jumphost": "runner"})
	require.NoError(t, err)

	var inv map[string]any
	require.NoError(t, yaml.Unmarshal(b, &inv))
	children := inv["all"].(map[string]any)["children"].(map[string]any)

	nodes := children["nodes"].(map[string]any)
	vars := nodes["vars"].(map[string]any)
	assert.Equal(t, "community.docker.docker_api", vars["ansible_connection"])
	assert.Equal(t, false, vars["ansible_become"], "node containers are root and have no sudo")
	assert.Equal(t, "root", vars["ansible_user"])
	cp := nodes["children"].(map[string]any)["control_plane"].(map[string]any)["hosts"].(map[string]any)
	assert.Contains(t, cp, "mozza-x-control-plane")
	assert.NotContains(t, cp, "mozza-x-worker")

	runner := children["runner"].(map[string]any)["hosts"].(map[string]any)["localhost"].(map[string]any)
	assert.Equal(t, "local", runner["ansible_connection"])

	alias := children["k8s-servers"].(map[string]any)["children"].(map[string]any)
	assert.Contains(t, alias, "control_plane")

	_, err = ansibleInventory(nil, map[string]string{"nodes": "runner"})
	require.Error(t, err, "a playbook group may not shadow a target name")
}

func TestAnsibleSpecValidation(t *testing.T) {
	_, err := Parse([]byte(`
name: demo
cluster: {provider: kind}
components:
  - name: cfg
    ansible: {playbook: site.yml, groups: {web: prod-servers}}
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a local target")

	src := t.TempDir()
	s, err := Parse([]byte(`
name: demo
source: {path: ` + src + `}
cluster: {provider: kind}
components:
  - name: cfg
    ansible: {playbook: ../elsewhere/site.yml}
`))
	require.NoError(t, err)
	_, err = Catalog{}.Resolve(s, ResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the source tree")
}

func TestAnsibleRunsInRunnerWithSourceReadOnly(t *testing.T) {
	src := t.TempDir()
	s, err := Parse([]byte(`
name: demo
source: {path: ` + src + `}
cluster: {provider: kind}
components:
  - name: cfg
    ansible:
      playbook: ansible/site.yml
      tags: [setup]
      limit: nodes
      env: {ANSIBLE_STDOUT_CALLBACK: yaml}
      extraVars: {pw: "${secret:demo/creds/pw}"}
secrets:
  - name: creds
    namespace: demo
    data: {pw: {generate: password}}
`))
	require.NoError(t, err)
	r, err := Catalog{}.Resolve(s, ResolveOptions{})
	require.NoError(t, err)

	fr := &fakeRunner{answers: map[string]string{
		"docker image inspect": "sha256:x",
		"kind get nodes":       "mozza-demo-control-plane\n",
		"docker exec":          "Python 3.11",
	}}
	e := NewEngine(r, fr, io.Discard, t.TempDir())
	require.NoError(t, os.MkdirAll(e.dir, 0o700))
	require.NoError(t, os.WriteFile(e.kubeconfigPath(),
		[]byte("clusters:\n- name: x\n  cluster:\n    server: https://127.0.0.1:41000\n"), 0o600))
	require.NoError(t, e.applySecretsForTest())

	require.NoError(t, e.runAnsible(context.Background(), r.Components[0]))

	run := fr.lines()[len(fr.lines())-1]
	for _, want := range []string{
		"docker run --rm --network kind",
		src + ":/src:ro",
		"-w /src/ansible",
		"ANSIBLE_STDOUT_CALLBACK=yaml",
		"ansible-playbook -i /work/inventory.yml -e @/work/extravars.yml --tags setup --limit nodes /src/ansible/site.yml",
	} {
		assert.Contains(t, run, want)
	}

	work := filepath.Join(e.dir, "ansible", "cfg")
	kc, err := os.ReadFile(filepath.Join(work, "kubeconfig"))
	require.NoError(t, err)
	assert.Contains(t, string(kc), "https://mozza-demo-control-plane:6443", "the runner reaches the API on the cluster network")

	vars, err := os.ReadFile(filepath.Join(work, "extravars.yml"))
	require.NoError(t, err)
	assert.NotContains(t, string(vars), "${secret:", "secret references resolve before the playbook runs")
	assert.NotContains(t, run, strings.TrimSpace(strings.TrimPrefix(string(vars), "pw: ")), "secrets stay off the command line")
}

func TestStackFileSourceIsRelativeToTheFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "app"), 0o755))
	file := filepath.Join(dir, "app.yaml")
	require.NoError(t, os.WriteFile(file, []byte(minimalStack+"source: {path: app}\n"), 0o644))

	s, err := Catalog{}.Load(file)
	require.NoError(t, err)
	r, err := Catalog{}.Resolve(s, ResolveOptions{})
	require.NoError(t, err)
	want, _ := filepath.EvalSymlinks(filepath.Join(dir, "app"))
	got, _ := filepath.EvalSymlinks(r.SourceDir)
	assert.Equal(t, want, got)
}

// applySecretsForTest generates and stores the stack's secrets the way `up`
// does, without the kubectl calls.
func (e *Engine) applySecretsForTest() error {
	st, err := openSecretStore(e.dir)
	if err != nil {
		return err
	}
	for _, sec := range e.stack.Secrets {
		if _, err := st.resolve(sec); err != nil {
			return err
		}
	}
	return st.save()
}
