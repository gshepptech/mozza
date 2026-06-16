package monitor

import (
	"time"

	"github.com/gshepptech/mozza/internal/store"
)

// MetricsResponse is the JSON response for the metrics endpoint.
type MetricsResponse struct {
	AppID      string            `json:"app_id"`
	Period     string            `json:"period"`
	Resolution string            `json:"resolution"`
	Data       []MetricDataPoint `json:"data"`
	Alerts     []Alert           `json:"alerts"`
}

// MetricDataPoint is a single data point in the metrics response.
type MetricDataPoint struct {
	Timestamp   int64   `json:"timestamp"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes int64   `json:"memory_bytes"`
	NetworkRx   int64   `json:"network_rx"`
	NetworkTx   int64   `json:"network_tx"`
}

// Alert represents a threshold violation alert.
type Alert struct {
	Type    string  `json:"type"`
	Message string  `json:"message"`
	Value   float64 `json:"value"`
}

// HealthResponse is the JSON response for the health endpoint.
type HealthResponse struct {
	AppID       string       `json:"app_id"`
	Status      HealthStatus `json:"status"`
	LastCheck   string       `json:"last_check,omitempty"`
	LastSuccess string       `json:"last_success,omitempty"`
	LastError   string       `json:"last_error,omitempty"`
}

// MonitoringSummary is the JSON response for the monitoring summary endpoint.
type MonitoringSummary struct {
	Apps []AppSummary `json:"apps"`
}

// AppSummary provides a high-level overview of one app's health.
type AppSummary struct {
	AppID  string       `json:"app_id"`
	Status HealthStatus `json:"status"`
}

const (
	// cpuAlertThreshold is the CPU usage percentage that triggers an alert.
	cpuAlertThreshold = 80.0
	// memAlertThreshold is the memory usage percentage that triggers an alert.
	memAlertThreshold = 80.0
)

// PeriodToDuration maps a human-readable period string to a duration.
func PeriodToDuration(period string) time.Duration {
	switch period {
	case "1h":
		return 1 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 1 * time.Hour
	}
}

// BuildMetricsResponse converts store metrics into the API response format.
// It also generates alerts for threshold violations based on the latest data.
func BuildMetricsResponse(appID string, period string, metrics []store.Metric) MetricsResponse {
	data := make([]MetricDataPoint, len(metrics))
	for i, m := range metrics {
		data[i] = MetricDataPoint{
			Timestamp:   m.Timestamp,
			CPUPercent:  m.CPUPercent,
			MemoryBytes: m.MemoryBytes,
			NetworkRx:   m.NetworkRx,
			NetworkTx:   m.NetworkTx,
		}
	}

	alerts := generateAlerts(metrics)

	return MetricsResponse{
		AppID:      appID,
		Period:     period,
		Resolution: "15s",
		Data:       data,
		Alerts:     alerts,
	}
}

// BuildHealthResponse converts an AppHealth into the API response format.
func BuildHealthResponse(appID string, h *AppHealth) HealthResponse {
	resp := HealthResponse{
		AppID:  appID,
		Status: StatusUnknown,
	}

	if h != nil {
		resp.Status = h.Status
		if !h.LastCheck.IsZero() {
			resp.LastCheck = h.LastCheck.Format(time.RFC3339)
		}
		if !h.LastSuccess.IsZero() {
			resp.LastSuccess = h.LastSuccess.Format(time.RFC3339)
		}
		resp.LastError = h.LastError
	}

	return resp
}

// generateAlerts checks the latest metrics for threshold violations.
func generateAlerts(metrics []store.Metric) []Alert {
	if len(metrics) == 0 {
		return nil
	}

	latest := metrics[len(metrics)-1]
	var alerts []Alert

	if latest.CPUPercent > cpuAlertThreshold {
		alerts = append(alerts, Alert{
			Type:    "cpu_high",
			Message: "CPU usage exceeds 80% threshold",
			Value:   latest.CPUPercent,
		})
	}

	// Memory alert requires knowing the limit. Since we only track usage bytes,
	// we alert if memory exceeds a high absolute value (4GB) as a safety net.
	// A proper percentage-based alert requires the container memory limit.
	const memAbsoluteThreshold = 4 * 1024 * 1024 * 1024 // 4 GiB
	if latest.MemoryBytes > memAbsoluteThreshold {
		alerts = append(alerts, Alert{
			Type:    "memory_high",
			Message: "Memory usage exceeds 4GiB threshold",
			Value:   float64(latest.MemoryBytes),
		})
	}

	return alerts
}
