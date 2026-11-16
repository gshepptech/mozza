package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/monitor"
)

// handleAppMetrics returns metrics for a specific app over a time period.
// Query params: period (1h|6h|24h|7d), metric (cpu|memory|network) — metric is
// informational only; all metric types are always returned.
func (s *Server) handleAppMetrics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Store == nil {
			Error(w, http.StatusServiceUnavailable, "store not configured")
			return
		}

		idStr := chi.URLParam(r, "id")
		appID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid app id")
			return
		}

		period := r.URL.Query().Get("period")
		if period == "" {
			period = "1h"
		}

		dur := monitor.PeriodToDuration(period)
		now := time.Now().Unix()
		start := now - int64(dur.Seconds())

		metrics, err := s.cfg.Store.QueryMetrics(r.Context(), appID, start, now)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to query metrics")
			return
		}

		resp := monitor.BuildMetricsResponse(idStr, period, metrics)
		JSON(w, http.StatusOK, resp)
	}
}

// handleAppHealth returns the current health status for a specific app.
func (s *Server) handleAppHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		appID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid app id")
			return
		}

		var health *monitor.AppHealth
		if s.monitorHealth != nil {
			health = s.monitorHealth.Status(appID)
		}

		resp := monitor.BuildHealthResponse(idStr, health)
		JSON(w, http.StatusOK, resp)
	}
}

// handleTimeSeries returns in-memory time-series data for a specific app.
// Query params: period (1h|6h|24h|7d).
func (s *Server) handleTimeSeries() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.tsStore == nil {
			Error(w, http.StatusServiceUnavailable, "time-series store not configured")
			return
		}

		appID := chi.URLParam(r, "id")
		if appID == "" {
			Error(w, http.StatusBadRequest, "app id is required")
			return
		}

		period := r.URL.Query().Get("period")
		if period == "" {
			period = "1h"
		}

		dur := monitor.PeriodToDuration(period)
		points := s.tsStore.Query(appID, dur)
		if points == nil {
			points = []monitor.TimePoint{}
		}

		JSON(w, http.StatusOK, monitor.TimeSeriesResponse{
			AppID:  appID,
			Period: period,
			Points: points,
		})
	}
}

// handleMonitoringSummary returns a health overview for all monitored apps.
func (s *Server) handleMonitoringSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var apps []monitor.AppSummary

		if s.monitorHealth != nil {
			statuses := s.monitorHealth.AllStatus()
			apps = make([]monitor.AppSummary, len(statuses))
			for i, st := range statuses {
				apps[i] = monitor.AppSummary{
					AppID:  strconv.FormatInt(st.AppID, 10),
					Status: st.Status,
				}
			}
		}

		if apps == nil {
			apps = []monitor.AppSummary{}
		}

		JSON(w, http.StatusOK, monitor.MonitoringSummary{Apps: apps})
	}
}
