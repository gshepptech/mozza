package monitor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthChecker_RegisterAndStatus(t *testing.T) {
	hc := NewHealthChecker()

	// No status before registration.
	assert.Nil(t, hc.Status(1))

	hc.RegisterApp(1, "deploy-1", "http://localhost:8080/health")

	status := hc.Status(1)
	require.NotNil(t, status)
	assert.Equal(t, StatusUnknown, status.Status)
	assert.Equal(t, int64(1), status.AppID)
	assert.Equal(t, "deploy-1", status.DeploymentID)
}

func TestHealthChecker_RegisterDuplicate(t *testing.T) {
	hc := NewHealthChecker()
	hc.RegisterApp(1, "deploy-1", "http://localhost:8080/health")
	hc.RegisterApp(1, "deploy-1", "http://localhost:8080/health")

	statuses := hc.AllStatus()
	assert.Len(t, statuses, 1)
}

func TestHealthChecker_UnregisterApp(t *testing.T) {
	hc := NewHealthChecker()
	hc.RegisterApp(1, "deploy-1", "http://localhost:8080/health")
	hc.UnregisterApp(1)

	assert.Nil(t, hc.Status(1))
	assert.Empty(t, hc.AllStatus())
}

func TestHealthChecker_HealthyAfterThreshold(t *testing.T) {
	hc := NewHealthChecker()

	// Create a test server that returns 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hc.RegisterApp(1, "deploy-1", srv.URL)

	// Simulate enough consecutive successes to reach healthy.
	for i := 0; i < healthyThreshold; i++ {
		hc.recordSuccess(1)
	}

	status := hc.Status(1)
	require.NotNil(t, status)
	assert.Equal(t, StatusHealthy, status.Status)
}

func TestHealthChecker_DegradedOnFailure(t *testing.T) {
	hc := NewHealthChecker()
	hc.RegisterApp(1, "deploy-1", "http://localhost:9999/health")

	hc.recordFailure(1, "connection refused")

	status := hc.Status(1)
	require.NotNil(t, status)
	assert.Equal(t, StatusDegraded, status.Status)
	assert.Equal(t, "connection refused", status.LastError)
}

func TestHealthChecker_DownAfterThreshold(t *testing.T) {
	hc := NewHealthChecker()
	hc.RegisterApp(1, "deploy-1", "http://localhost:9999/health")

	for i := 0; i < downThreshold; i++ {
		hc.recordFailure(1, "connection refused")
	}

	status := hc.Status(1)
	require.NotNil(t, status)
	assert.Equal(t, StatusDown, status.Status)
}

func TestHealthChecker_RecoveryFromDown(t *testing.T) {
	hc := NewHealthChecker()
	hc.RegisterApp(1, "deploy-1", "http://localhost:9999/health")

	// Go down.
	for i := 0; i < downThreshold; i++ {
		hc.recordFailure(1, "connection refused")
	}
	assert.Equal(t, StatusDown, hc.Status(1).Status)

	// Recover.
	for i := 0; i < healthyThreshold; i++ {
		hc.recordSuccess(1)
	}

	status := hc.Status(1)
	require.NotNil(t, status)
	assert.Equal(t, StatusHealthy, status.Status)
	assert.Equal(t, 0, status.ConsecutiveFail)
}

func TestHealthChecker_AllStatus(t *testing.T) {
	hc := NewHealthChecker()
	hc.RegisterApp(1, "deploy-1", "http://localhost:8080/health")
	hc.RegisterApp(2, "deploy-2", "http://localhost:8081/health")

	statuses := hc.AllStatus()
	assert.Len(t, statuses, 2)
}
