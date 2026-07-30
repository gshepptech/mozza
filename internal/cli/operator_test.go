package cli

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/deploy"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/store"
)

// operatorRecipe is a minimal recipe that parses, builds and validates, so
// executeDeployment reaches the deployer call.
const operatorRecipe = `App: warntest

Api:
  from image nginx:alpine
  open to the public on port 8080
  health check /healthz
  run 1 copy
`

// TestExecuteDeployment_LogsWarningsOnce covers the operator path. The operator
// writes the same deployments.log the dashboard reads, so a local deploy picked
// up here has to report the gaps the CLI reports — and report each one once.
func TestExecuteDeployment_LogsWarningsOnce(t *testing.T) {
	t.Parallel()

	st, deployID := operatorDeploymentFixture(t)

	warnings := []string{
		"api: Permissions are Kubernetes-only. Skipped for local target.",
		"api: Auto-scaling is Kubernetes-only. Running a fixed replica count locally.",
	}
	executeDeployment(t.Context(), st, &stubOperatorDeployer{warnings: warnings},
		deployID, operatorRecipe, "local", "production")

	dep, err := st.DeploymentByID(deployID)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(dep.Log, "2 warning(s):"), "log:\n%s", dep.Log)
	for _, w := range warnings {
		assert.Equal(t, 1, strings.Count(dep.Log, "  ! "+w),
			"expected %q exactly once in log:\n%s", w, dep.Log)
	}
}

// TestExecuteDeployment_NoWarningsLogsNothing keeps silence meaningful on the
// operator path too.
func TestExecuteDeployment_NoWarningsLogsNothing(t *testing.T) {
	t.Parallel()

	st, deployID := operatorDeploymentFixture(t)

	executeDeployment(t.Context(), st, &stubOperatorDeployer{},
		deployID, operatorRecipe, "local", "production")

	dep, err := st.DeploymentByID(deployID)
	require.NoError(t, err)

	assert.NotContains(t, dep.Log, "warning(s):")
	assert.NotContains(t, dep.Log, "  ! ")
}

// operatorDeploymentFixture opens a store and creates the user, team and recipe
// rows a deployment's foreign keys require.
func operatorDeploymentFixture(t *testing.T) (*store.Store, string) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	u, err := st.CreateUser("operator-warnings@example.com", "Operator", "password123", "admin")
	require.NoError(t, err)
	team, err := st.CreateTeam("Warn Team", "warn-team", u.ID)
	require.NoError(t, err)
	rec, err := st.CreateRecipe(team.ID, "warntest", operatorRecipe, "{}", u.ID)
	require.NoError(t, err)

	dep, err := st.CreateDeployment(rec.ID, team.ID, "local", "production", u.ID, "")
	require.NoError(t, err)

	return st, dep.ID
}

// stubOperatorDeployer is a deploy.Deployer that succeeds and reports the
// warnings it was given, standing in for a local deploy without needing Docker.
type stubOperatorDeployer struct {
	warnings []string
}

var _ deploy.Deployer = (*stubOperatorDeployer)(nil)

func (d *stubOperatorDeployer) Deploy(
	_ context.Context, _ *plan.AppPlan, _ deploy.DeployOptions,
) (*deploy.DeployResult, error) {
	return &deploy.DeployResult{
		DeployID: "stub-deploy",
		Status:   deploy.StatusSuccess,
		Warnings: d.warnings,
	}, nil
}

func (d *stubOperatorDeployer) Rollback(_ context.Context, _ string) error { return nil }

func (d *stubOperatorDeployer) Status(_ context.Context, _ string) (*deploy.AppStatus, error) {
	return &deploy.AppStatus{}, nil
}

func (d *stubOperatorDeployer) Logs(
	_ context.Context, _ string, _ deploy.LogOptions,
) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (d *stubOperatorDeployer) Down(_ context.Context, _ string, _ deploy.DownOptions) error {
	return nil
}
