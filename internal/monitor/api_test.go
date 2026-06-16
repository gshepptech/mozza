package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gshepptech/mozza/internal/store"
)

func TestPeriodToDuration(t *testing.T) {
	tests := []struct {
		period   string
		expected time.Duration
	}{
		{"1h", 1 * time.Hour},
		{"6h", 6 * time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"unknown", 1 * time.Hour}, // default
		{"", 1 * time.Hour},        // default
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			result := PeriodToDuration(tt.period)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildMetricsResponse(t *testing.T) {
	metrics := []store.Metric{
		{
			Timestamp:   1000,
			CPUPercent:  50.0,
			MemoryBytes: 1024,
			NetworkRx:   100,
			NetworkTx:   200,
		},
		{
			Timestamp:   1015,
			CPUPercent:  60.0,
			MemoryBytes: 2048,
			NetworkRx:   150,
			NetworkTx:   250,
		},
	}

	resp := BuildMetricsResponse("app-1", "1h", metrics)

	assert.Equal(t, "app-1", resp.AppID)
	assert.Equal(t, "1h", resp.Period)
	assert.Equal(t, "15s", resp.Resolution)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, int64(1000), resp.Data[0].Timestamp)
	assert.Equal(t, 50.0, resp.Data[0].CPUPercent)
}

func TestBuildMetricsResponseEmpty(t *testing.T) {
	resp := BuildMetricsResponse("app-1", "1h", nil)

	assert.Equal(t, "app-1", resp.AppID)
	assert.Empty(t, resp.Data)
	assert.Nil(t, resp.Alerts)
}

func TestGenerateAlerts_CPUHigh(t *testing.T) {
	metrics := []store.Metric{
		{CPUPercent: 85.0, MemoryBytes: 1024},
	}

	alerts := generateAlerts(metrics)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "cpu_high", alerts[0].Type)
	assert.Equal(t, 85.0, alerts[0].Value)
}

func TestGenerateAlerts_NoAlerts(t *testing.T) {
	metrics := []store.Metric{
		{CPUPercent: 50.0, MemoryBytes: 1024},
	}

	alerts := generateAlerts(metrics)
	assert.Nil(t, alerts)
}

func TestGenerateAlerts_Empty(t *testing.T) {
	alerts := generateAlerts(nil)
	assert.Nil(t, alerts)
}

func TestBuildHealthResponse(t *testing.T) {
	now := time.Now()
	h := &AppHealth{
		Status:    StatusHealthy,
		LastCheck: now,
	}

	resp := BuildHealthResponse("app-1", h)
	assert.Equal(t, "app-1", resp.AppID)
	assert.Equal(t, StatusHealthy, resp.Status)
	assert.NotEmpty(t, resp.LastCheck)
}

func TestBuildHealthResponseNil(t *testing.T) {
	resp := BuildHealthResponse("app-1", nil)
	assert.Equal(t, "app-1", resp.AppID)
	assert.Equal(t, StatusUnknown, resp.Status)
}
