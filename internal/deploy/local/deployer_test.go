package local

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/deploy"
	localcompose "github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/store"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	st, err := store.Open(dbPath)
	require.NoError(t, err)
	defer st.Close()

	d := New(st, tmpDir)

	assert.NotNil(t, d, "New should return a non-nil Deployer")
	assert.Equal(t, tmpDir, d.projectDir)
	assert.Equal(t, st, d.store)
}

func TestNew_PreservesProjectDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
	}{
		{name: "absolute path", projectDir: "/tmp/my-project"},
		{name: "relative path", projectDir: "my-project"},
		{name: "empty path", projectDir: ""},
		{name: "nested path", projectDir: "/home/user/projects/app/v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			dbPath := tmpDir + "/test.db"

			st, err := store.Open(dbPath)
			require.NoError(t, err)
			defer st.Close()

			d := New(st, tt.projectDir)
			assert.Equal(t, tt.projectDir, d.projectDir)
		})
	}
}

// TestCompilePlan_ReturnsWarnings checks that the deploy path returns the same
// capability warnings the CLI reports, so a dashboard or operator deploy is not
// silent about them. Deploy itself needs Docker, so the compile step is tested
// directly.
func TestCompilePlan_ReturnsWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slice        plan.Slice
		wantWarnings []string
	}{
		{
			name: "kubernetes-only permissions",
			slice: plan.Slice{
				Name: "api", Kind: plan.SliceKindAPI, Image: "nginx:alpine", Port: 8080, Replicas: 1,
				Permissions: []plan.Permission{{Verbs: []string{"get"}, Resources: []string{"pods"}}},
			},
			wantWarnings: []string{"api: Permissions are Kubernetes-only. Skipped for local target."},
		},
		{
			name: "kubernetes-only auto-scaling",
			slice: plan.Slice{
				Name: "api", Kind: plan.SliceKindAPI, Image: "nginx:alpine", Port: 8080, Replicas: 1,
				AutoScale: &plan.AutoScaleSpec{MinReplicas: 2, MaxReplicas: 10, CPUTarget: 80},
			},
			wantWarnings: []string{"api: Auto-scaling is Kubernetes-only. Running a fixed replica count locally."},
		},
		{
			name: "tcp-only probe compose cannot run",
			slice: plan.Slice{
				Name: "api", Kind: plan.SliceKindAPI, Image: "nginx:alpine", Port: 8080, Replicas: 1,
				Probes: []plan.ProbeSpec{{Type: "readiness", TCPPort: 9090}},
			},
			wantWarnings: []string{
				"api: Compose has no TCP health check, so the readiness check on TCP port 9090 was dropped. " +
					"Give the check an HTTP path or a command to have Compose run it.",
			},
		},
		{
			name: "several gaps in one slice",
			slice: plan.Slice{
				Name: "worker", Kind: plan.SliceKindWorker, Image: "worker:v1", Replicas: 1,
				DaemonMode:    true,
				NetworkPolicy: &plan.NetworkPolicySpec{DenyAll: true},
			},
			wantWarnings: []string{
				"worker: Network policies are Kubernetes-only. All services can communicate locally.",
				"worker: DaemonSet mode requires Docker Swarm. Running as single instance.",
			},
		},
		{
			name: "plain slice loses nothing",
			slice: plan.Slice{
				Name: "api", Kind: plan.SliceKindAPI, Image: "nginx:alpine", Port: 8080, Replicas: 1,
				HealthPath: "/healthz",
			},
			wantWarnings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			composeDir := t.TempDir()
			p := &plan.AppPlan{Name: "warntest", Slices: []plan.Slice{tt.slice}}

			warnings, err := compilePlan(t.Context(), composeDir, p)
			require.NoError(t, err)

			assert.Equal(t, tt.wantWarnings, warnings)
			assert.FileExists(t, filepath.Join(composeDir, "docker-compose.yml"),
				"compilePlan must still write the file docker compose up reads")
		})
	}
}

// TestDeploy_ReturnsWarnings checks that DeployResult.Warnings is populated.
// TestCompilePlan_ReturnsWarnings covers the helper, not the field assignment on
// the returned result, so this test is what fails if that assignment goes.
//
// Both outcomes are covered. A bad image tag or a port clash is a failure users
// hit often, and the warnings matter there as much as on success.
func TestDeploy_ReturnsWarnings(t *testing.T) {
	requireDockerCompose(t)

	wantWarnings := []string{
		"api: Permissions are Kubernetes-only. Skipped for local target.",
		"api: DaemonSet mode requires Docker Swarm. Running as single instance.",
	}

	// A slice that both compiles with warnings and, in the failure case, cannot
	// start. The .invalid TLD is reserved by RFC 2606, so the pull fails fast
	// rather than waiting on a real registry.
	newPlan := func(name, image string) *plan.AppPlan {
		return &plan.AppPlan{Name: name, Slices: []plan.Slice{{
			Name: "api", Kind: plan.SliceKindAPI, Image: image, Replicas: 1,
			Permissions: []plan.Permission{{Verbs: []string{"get"}, Resources: []string{"pods"}}},
			DaemonMode:  true,
		}}}
	}

	tests := []struct {
		name string
		// app names the compose project, so it must be distinct per case: two
		// cases sharing one would tear down each other's containers.
		app        string
		image      string
		wantStatus deploy.DeployStatus
		wantErr    bool
	}{
		{
			name:       "successful deploy reports what the target could not carry",
			app:        "warnwire-ok",
			image:      "nginx:alpine",
			wantStatus: deploy.StatusSuccess,
			wantErr:    false,
		},
		{
			name:       "failed deploy still reports it",
			app:        "warnwire-fail",
			image:      "mozza-test-nonexistent.invalid/nope:v0",
			wantStatus: deploy.StatusFailed,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			st, err := store.Open(filepath.Join(projectDir, "test.db"))
			require.NoError(t, err)
			defer st.Close()

			p := newPlan(tt.app, tt.image)
			d := New(st, projectDir)

			t.Cleanup(func() { composeDown(t, filepath.Join(projectDir, tt.app)) })

			result, err := d.Deploy(t.Context(), p, deploy.DeployOptions{TriggeredBy: "test"})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// The point of the test: a result comes back either way, and it
			// carries the warnings.
			require.NotNil(t, result, "Deploy must return a result carrying warnings, even on failure")
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, wantWarnings, result.Warnings)
		})
	}
}

// requireDockerCompose skips when Docker is unusable, so the suite still runs on
// a machine without it. These two cases need a real compose run because the
// assignment under test is on Deploy's own return value.
func requireDockerCompose(t *testing.T) {
	t.Helper()

	if _, err := localcompose.NewRunner(t.TempDir()); err != nil {
		t.Skipf("docker compose not available: %v", err)
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon not usable: %v", err)
	}
}

// composeDown tears down whatever the test started, so a failed assertion cannot
// leave containers behind.
//
// Uses a fresh context rather than t.Context(): that one is cancelled just before
// cleanup functions run, so passing it here makes the teardown fail silently and
// leak a running container.
func composeDown(t *testing.T, composeDir string) {
	t.Helper()

	runner, err := localcompose.NewRunner(composeDir)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var buf bytes.Buffer
	if err := runner.Down(ctx, &buf); err != nil {
		t.Logf("compose down: %v: %s", err, buf.String())
	}
}

func TestRollback_NoPreviousDeploy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	st, err := store.Open(dbPath)
	require.NoError(t, err)
	defer st.Close()

	d := New(st, tmpDir)

	// Rollback with no previous deploy should return an error.
	err = d.Rollback(t.Context(), "nonexistent-app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no previous deployment")
}
