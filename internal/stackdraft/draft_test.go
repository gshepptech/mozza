package stackdraft

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/stack"
)

func TestOverlayScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		file string
		pick bool
	}{
		{"values-dev.yaml", true},
		{"values-demo.yaml", true},
		{"values-dev-k3s.yaml", true},
		{"values-local.yaml", true},
		{"values-prod.yaml", false},
		{"values-prod-k3s-local.yaml", false},
		{"values-test-digests.yaml", false},
		{"values-hardened.yaml", false},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.pick, overlayScore(tt.file) > 0)
		})
	}
}

func TestSecretClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		key        string
		comment    string
		names      bool // a reference to a Secret, never generated
		generator  string
		documented bool
	}{
		{"age key from comment", "masterKey", "generate with age-keygen; fail-fasts without it", false, "age-key", true},
		{"base64 from comment", "kek", "KEK (base64 32-byte AES). REFUSE TO BOOT without it", false, "base64", true},
		{"password", "adminPassword", "", false, "password", false},
		{"plain secret", "jwtSecret", "helm install fails fast if not provided", false, "hex", true},
		{"conditional is not documented", "apiKey", "required when executionMode=judge0", false, "hex", false},
		{"existing-secret reference", "authExistingSecret", "", true, "", false},
		{"pull secret reference", "pullSecret", "", true, "", false},
		{"reference by comment", "tokenSecret", "Name of an existing Secret holding the token", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.names, namesASecret(tt.key, tt.comment))
			if tt.names {
				return
			}
			f := classify("a."+tt.key, tt.key, tt.comment)
			assert.Equal(t, tt.generator, f.generator)
			assert.Equal(t, tt.documented, f.documented)
		})
	}
}

func TestFindImagesSkipsDisabledBlocks(t *testing.T) {
	t.Parallel()
	got := findImages(map[string]any{
		"image": map[string]any{"repository": "ghcr.io/org/app", "tag": "1.0", "pullPolicy": "Always"},
		"web":   map[string]any{"enabled": false, "image": map[string]any{"repository": "org/web", "tag": "1"}},
		"agent": map[string]any{"image": "org/agent:2.0"},
	}, "")
	require.Len(t, got, 2)
	assert.Equal(t, "agent.image", got[0].path)
	assert.True(t, got[0].asString)
	assert.Equal(t, "image", got[1].path)
	assert.True(t, got[1].pullPolicy)
	assert.Equal(t, "app", got[1].shortName())
}

func TestMatchScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		image, df string
		want      int
	}{
		{"storefront-server", "storefront-server", 4},
		{"inventory-web", "web", 3},
		{"inventory", "inventory", 4},
		{"postgres", "web", 0},
	}
	for _, tt := range tests {
		t.Run(tt.image+"/"+tt.df, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, matchScore(tt.image, tt.df, "inventory", "inventory"))
		})
	}
}

func TestPublicRegistry(t *testing.T) {
	t.Parallel()
	for ref, want := range map[string]bool{
		"postgres":                   true,
		"axllent/mailpit":            true,
		"docker.io/library/docker":   true,
		"quay.io/org/x":              true,
		"ghcr.io/org/private":        false,
		"registry.gitlab.com/org/x":  false,
		"nexus.example.com:5000/app": false,
	} {
		assert.Equal(t, want, publicRegistry(ref), ref)
	}
}

func TestSuggestTarget(t *testing.T) {
	t.Parallel()
	for host, want := range map[string]string{
		"k8s-servers": "control_plane",
		"k8s-workers": "workers",
		"jumphost":    "runner",
		"localhost":   "runner",
		"nexus":       "nodes",
		"all":         "",
	} {
		assert.Equal(t, want, suggestTarget(host), host)
	}
}

// writeTree lays out files under a temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "shopfront")
	for p, body := range files {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}
	return root
}

func TestDraftProducesAStackThatLoads(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{
		"Dockerfile":                        "FROM scratch\n",
		"Dockerfile.worker":                 "FROM scratch\n",
		"deploy/shopfront/Chart.yaml":       "apiVersion: v2\nname: shopfront\nversion: 0.1.0\n",
		"deploy/shopfront/values-dev.yaml":  "replicaCount: 1\n",
		"deploy/shopfront/values-prod.yaml": "replicaCount: 3\n",
		"deploy/shopfront/templates/x.yaml": "apiVersion: postgresql.cnpg.io/v1\nkind: Cluster\n",
		"deploy/shopfront-crds/Chart.yaml":  "apiVersion: v2\nname: shopfront-crds\nversion: 0.1.0\n",
		"ansible/playbooks/site.yml":        "- hosts: k8s-servers\n  tasks: []\n",
		"deploy/shopfront/values.yaml": `
image:
  repository: nginx
  tag: ""
  pullPolicy: Always
worker:
  image:
    repository: ghcr.io/acme/shopfront-worker
    tag: "1.2"
persistence:
  storageClass: local-path
auth:
  # -- Signing key. The server refuses to start without it.
  signingKey: ""
  existingSecret: ""
oidc:
  enabled: false
  clientSecret: ""
`,
	})

	res, err := Draft(root, Options{OutputDir: root})
	require.NoError(t, err)

	s, err := stack.Parse(res.YAML)
	require.NoError(t, err, string(res.YAML))
	assert.Equal(t, "shopfront", s.Name)
	assert.Equal(t, ".", s.Source.Path, "a file written into the tree points at the tree relatively")

	refs := map[string]string{}
	for _, img := range s.Images {
		refs[img.Ref] = img.Build.Dockerfile
	}
	assert.Equal(t, "Dockerfile", refs["shopfront:mozza"], "the root Dockerfile builds the chart's main image")
	assert.Equal(t, "Dockerfile.worker", refs["shopfront-worker:mozza"])

	require.Len(t, s.Secrets, 1)
	assert.Contains(t, s.Secrets[0].Data, "auth.signingKey")
	assert.NotContains(t, s.Secrets[0].Data, "oidc.clientSecret", "a disabled feature's credential is left alone")
	assert.NotContains(t, s.Secrets[0].Data, "auth.existingSecret")

	app := s.Components[len(s.Components)-1]
	assert.Equal(t, []string{"deploy/shopfront/values-dev.yaml"}, app.Helm.ValuesFiles)
	assert.Equal(t, "${storageClass}", app.Values["persistence"].(map[string]any)["storageClass"])
	assert.Equal(t, "cnpg", s.Components[0].Use, "without a render, addons come from the templates")

	text := string(res.YAML)
	assert.Contains(t, text, `Also found chart "shopfront-crds"`)
	assert.Contains(t, text, "values-prod.yaml was not used")
	assert.Contains(t, text, "playbook: ansible/playbooks/site.yml")
	assert.Contains(t, text, "k8s-servers: control_plane")
}

func TestDraftWithoutAChartPointsAtImport(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{"docker-compose.yml": "services: {}\n"})
	_, err := Draft(root, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mozza import docker-compose.yml")
}

func TestDraftIgnoresDependencyCharts(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{
		"chart/Chart.yaml":              "apiVersion: v2\nname: app\nversion: 0.1.0\ndependencies: [{name: redis, version: 1.0.0}]\n",
		"chart/values.yaml":             "{}\n",
		"chart/charts/redis/Chart.yaml": "apiVersion: v2\nname: redis\nversion: 1.0.0\n",
	})
	res, err := Draft(root, Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(res.YAML), `"redis"`)
	assert.Contains(t, string(res.YAML), "1 dependencies")
}
