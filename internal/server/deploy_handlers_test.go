package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/deploy"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/store"
)

// deployableRecipe is a minimal recipe that parses, builds and validates, so
// runDeployWith reaches the deployer call.
const deployableRecipe = `App: warntest

Api:
  from image nginx:alpine
  open to the public on port 8080
  health check /healthz
  run 1 copy
`

// TestWriteLogEvents covers two framing rules, each of which would otherwise
// reach the browser as lost log lines.
//
// First, a multi-line chunk must not go out as one "data:" payload: the SSE
// grammar treats every line after the first as an unknown field and drops it.
//
// Second, framing on "\n" alone is incomplete: SSE terminates a line on
// CR, LF or CRLF, so a bare CR in log text escaped the "data: " prefix and the
// remainder was parsed as arbitrary SSE fields. That is reachable — the log
// interpolates the unvalidated deployment environment — and an injected
// "event: done" makes the client close the stream before the warnings arrive.
func TestWriteLogEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chunk string
		want  string
	}{
		{
			name:  "every line of a multi-line chunk gets its own event",
			chunk: "3 warning(s):\n  ! api: one\n  ! api: two\nStep 5/5: Waiting...\n",
			want: "data: 3 warning(s):\n\n" +
				"data:   ! api: one\n\n" +
				"data:   ! api: two\n\n" +
				"data: Step 5/5: Waiting...\n\n",
		},
		{
			name:  "trailing newline does not produce an empty event",
			chunk: "Step 1/5: Parsing recipe...\n",
			want:  "data: Step 1/5: Parsing recipe...\n\n",
		},
		{
			name:  "line without a trailing newline is still sent",
			chunk: "Step 1/5: Parsing recipe...",
			want:  "data: Step 1/5: Parsing recipe...\n\n",
		},
		{
			name:  "blank lines inside a chunk are skipped",
			chunk: "first\n\nsecond\n",
			want:  "data: first\n\ndata: second\n\n",
		},
		{
			name:  "empty chunk sends nothing",
			chunk: "",
			want:  "",
		},
		{
			// SSE terminates a line on CR, LF or CRLF. A bare CR that stayed
			// inside the payload would carry the rest past the "data: " prefix,
			// where a reader parses it as further SSE fields.
			name:  "bare carriage return is a line break, not payload",
			chunk: "prod\revent: done\rdata: FORGED\n",
			want: "data: prod\n\n" +
				"data: event: done\n\n" +
				"data: data: FORGED\n\n",
		},
		{
			name:  "CRLF is a single break, not two",
			chunk: "Step 1/5: Parsing recipe...\r\nStep 2/5: Building plan...\r\n",
			want: "data: Step 1/5: Parsing recipe...\n\n" +
				"data: Step 2/5: Building plan...\n\n",
		},
		{
			name:  "mixed terminators in one chunk",
			chunk: "one\rtwo\r\nthree\nfour",
			want:  "data: one\n\ndata: two\n\ndata: three\n\ndata: four\n\n",
		},
		{
			name:  "runs of terminators collapse without empty events",
			chunk: "\r\n\n\rfirst\r\r\n\nsecond\n\r",
			want:  "data: first\n\ndata: second\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			writeLogEvents(rec, rec, tt.chunk)
			body := rec.Body.String()

			assert.Equal(t, tt.want, body)

			// The framing invariant, independent of the expectation above: no
			// content may reach the wire outside a "data: " field. Any line that
			// escaped the prefix would be parsed as an SSE field name, which is
			// exactly how an injected "event: done" would close a client's stream.
			//
			// The split must use the reader's notion of a line — CR, LF or CRLF —
			// and the predicate is spelled out here rather than reusing the
			// production isSSELineTerminator on purpose. Splitting on "\n" alone
			// made this check blind to the very defect it exists to catch: a
			// CR-escaped field stays inside one LF-delimited segment that still
			// begins with "data: ". Sharing the production predicate would let the
			// check inherit that mistake again if the predicate itself were wrong.
			for line := range strings.FieldsFuncSeq(body, func(r rune) bool {
				return r == '\n' || r == '\r'
			}) {
				assert.True(t, strings.HasPrefix(line, "data: "),
					"line %q reached the wire without a data: prefix", line)
			}
		})
	}
}

// TestRequestLogger_PreservesOptionalInterfaces guards the transport underneath
// every streaming endpoint. requestLogger wraps each request in a
// *responseRecorder, which embeds the http.ResponseWriter *interface* — so only
// Header/Write/WriteHeader are promoted and any optional interface a handler
// type-asserts for is silently lost.
//
// Without Flush, handleDeploymentLogs' w.(http.Flusher) fails and SSE answers
// 500, so TestWriteLogEvents could pass while the endpoint streamed nothing.
// Without Hijack, a websocket upgrade fails the same way.
func TestRequestLogger_PreservesOptionalInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// asserts reports whether the handler saw the interface it needs.
		asserts func(w http.ResponseWriter) bool
		writer  func(rec *httptest.ResponseRecorder) http.ResponseWriter
		name    string
	}{
		{
			name:    "http.Flusher survives, as handleDeploymentLogs needs",
			asserts: func(w http.ResponseWriter) bool { _, ok := w.(http.Flusher); return ok },
			writer:  func(rec *httptest.ResponseRecorder) http.ResponseWriter { return rec },
		},
		{
			name:    "http.Hijacker survives, as websocket.Accept needs",
			asserts: func(w http.ResponseWriter) bool { _, ok := w.(http.Hijacker); return ok },
			// httptest.ResponseRecorder is not a Hijacker, so the underlying
			// writer has to supply it for the promotion to be worth testing.
			writer: func(rec *httptest.ResponseRecorder) http.ResponseWriter {
				return hijackableWriter{ResponseRecorder: rec}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var saw bool
			handler := requestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				saw = tt.asserts(w)
			}))

			handler.ServeHTTP(tt.writer(httptest.NewRecorder()),
				httptest.NewRequest(http.MethodGet, "/", nil))

			assert.True(t, saw, "handler lost an optional interface through requestLogger")
		})
	}
}

// hijackableWriter is an httptest.ResponseRecorder that also implements
// http.Hijacker, standing in for the real connection-backed writer.
type hijackableWriter struct {
	*httptest.ResponseRecorder
}

func (hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errNotHijackable
}

var errNotHijackable = errors.New("hijack not supported in tests")

// TestRunDeployWith_LogsWarningsOnce covers the dashboard path. runDeployWith is
// the sole chokepoint for all five server deploy routes, so proving it here
// proves them all. Each warning must appear exactly once — the deploy log is the
// only place the dashboard reports them, and a second render would read as two
// separate gaps.
func TestRunDeployWith_LogsWarningsOnce(t *testing.T) {
	t.Parallel()

	st, dep := deploymentFixture(t)
	s := &Server{cfg: Config{Store: st}}

	stub := &stubDeployer{warnings: []string{
		"api: Permissions are Kubernetes-only. Skipped for local target.",
		"api: Auto-scaling is Kubernetes-only. Running a fixed replica count locally.",
	}}

	s.runDeployWith(stub, dep.ID, deployableRecipe, "local", "production")

	updated, err := st.DeploymentByID(dep.ID)
	require.NoError(t, err)

	for _, want := range append([]string{"2 warning(s):"}, prefixed(stub.warnings)...) {
		assert.Equal(t, 1, strings.Count(updated.Log, want),
			"expected %q exactly once in log:\n%s", want, updated.Log)
	}
}

// TestRunDeployWith_NoWarningsLogsNothing keeps silence meaningful: a deploy
// that lost nothing must not log an empty warning header.
func TestRunDeployWith_NoWarningsLogsNothing(t *testing.T) {
	t.Parallel()

	st, dep := deploymentFixture(t)
	s := &Server{cfg: Config{Store: st}}

	s.runDeployWith(&stubDeployer{}, dep.ID, deployableRecipe, "local", "production")

	updated, err := st.DeploymentByID(dep.ID)
	require.NoError(t, err)

	assert.NotContains(t, updated.Log, "warning(s):")
	assert.NotContains(t, updated.Log, "  ! ")
}

// prefixed renders warnings the way the log does, so the assertion checks the
// rendered line rather than the bare warning text.
func prefixed(warnings []string) []string {
	lines := make([]string, len(warnings))
	for i, w := range warnings {
		lines[i] = "  ! " + w
	}
	return lines
}

// deploymentFixture opens a store and creates the user, team and recipe rows a
// deployment's foreign keys require.
func deploymentFixture(t *testing.T) (*store.Store, *store.Deployment) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	u, err := st.CreateUser("deploy-warnings@example.com", "Deployer", "password123", "admin")
	require.NoError(t, err)
	team, err := st.CreateTeam("Warn Team", "warn-team", u.ID)
	require.NoError(t, err)
	rec, err := st.CreateRecipe(team.ID, "warntest", deployableRecipe, "{}", u.ID)
	require.NoError(t, err)

	dep, err := st.CreateDeployment(rec.ID, team.ID, "local", "production", u.ID, "")
	require.NoError(t, err)

	return st, dep
}

// stubDeployer is a deploy.Deployer that succeeds and reports the warnings it
// was given, standing in for a local deploy without needing Docker.
type stubDeployer struct {
	warnings []string
}

var _ deploy.Deployer = (*stubDeployer)(nil)

func (d *stubDeployer) Deploy(_ context.Context, _ *plan.AppPlan, _ deploy.DeployOptions) (*deploy.DeployResult, error) {
	return &deploy.DeployResult{
		DeployID: "stub-deploy",
		Status:   deploy.StatusSuccess,
		Warnings: d.warnings,
	}, nil
}

func (d *stubDeployer) Rollback(_ context.Context, _ string) error { return nil }

func (d *stubDeployer) Status(_ context.Context, _ string) (*deploy.AppStatus, error) {
	return &deploy.AppStatus{}, nil
}

func (d *stubDeployer) Logs(_ context.Context, _ string, _ deploy.LogOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (d *stubDeployer) Down(_ context.Context, _ string, _ deploy.DownOptions) error { return nil }
