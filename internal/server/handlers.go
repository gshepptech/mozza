package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/version"
)

// healthResponse is the JSON structure returned by the health endpoint.
type healthResponse struct {
	Status string `json:"status"`
}

// versionResponse is the JSON structure returned by the version endpoint.
type versionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// planResponse is the JSON structure returned by the plan endpoint.
type planResponse struct {
	Name        string               `json:"name"`
	Slices      []sliceResponse      `json:"slices"`
	Ingredients []ingredientResponse `json:"ingredients"`
}

// sliceResponse is the JSON representation of a single deployment slice.
type sliceResponse struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Image      string `json:"image"`
	Port       int    `json:"port"`
	Public     bool   `json:"public"`
	Replicas   int    `json:"replicas"`
	HealthPath string `json:"health_path,omitempty"`
	// Expanded fields.
	Ports            []portResponse            `json:"ports,omitempty"`
	Probes           []probeResponse           `json:"probes,omitempty"`
	InitSteps        []initStepResponse        `json:"init_steps,omitempty"`
	Sidecars         []sidecarResponse         `json:"sidecars,omitempty"`
	Mounts           []mountResponse           `json:"mounts,omitempty"`
	Env              map[string]string         `json:"env,omitempty"`
	Schedule         string                    `json:"schedule,omitempty"`
	RunOnce          bool                      `json:"run_once,omitempty"`
	Parallelism      int                       `json:"parallelism,omitempty"`
	Retries          int                       `json:"retries,omitempty"`
	DaemonMode       bool                      `json:"daemon_mode,omitempty"`
	OrderedStartup   bool                      `json:"ordered_startup,omitempty"`
	PeerDiscovery    bool                      `json:"peer_discovery,omitempty"`
	StatefulStorage  string                    `json:"stateful_storage,omitempty"`
	DNSName          string                    `json:"dns_name,omitempty"`
	ServiceAccount   string                    `json:"service_account,omitempty"`
	GracefulShutdown int                       `json:"graceful_shutdown,omitempty"`
	RestartPolicy    string                    `json:"restart_policy,omitempty"`
	Domain           string                    `json:"domain,omitempty"`
	AutoScale        *autoScaleResponse        `json:"auto_scale,omitempty"`
	DisruptionBudget *disruptionBudgetResponse `json:"disruption_budget,omitempty"`
	Security         *securityResponse         `json:"security,omitempty"`
	Permissions      []permissionResponse      `json:"permissions,omitempty"`
	Scheduling       *schedulingResponse       `json:"scheduling,omitempty"`
	NetworkPolicy    *networkPolicyResponse    `json:"network_policy,omitempty"`
	Lifecycle        *lifecycleResponse        `json:"lifecycle,omitempty"`
	UpdateStrategy   *updateStrategyResponse   `json:"update_strategy,omitempty"`
}

// portResponse is the JSON representation of a named port.
type portResponse struct {
	Name     string `json:"name,omitempty"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
}

// probeResponse is the JSON representation of a health/readiness probe.
type probeResponse struct {
	Type     string `json:"type"`
	HTTPPath string `json:"http_path,omitempty"`
	Command  string `json:"command,omitempty"`
	TCPPort  int    `json:"tcp_port,omitempty"`
	Interval int    `json:"interval,omitempty"`
	Timeout  int    `json:"timeout,omitempty"`
	Delay    int    `json:"delay,omitempty"`
}

// initStepResponse is the JSON representation of an init container step.
type initStepResponse struct {
	Image   string `json:"image"`
	Command string `json:"command"`
}

// sidecarResponse is the JSON representation of a sidecar container.
type sidecarResponse struct {
	Name  string         `json:"name"`
	Image string         `json:"image"`
	Ports []portResponse `json:"ports,omitempty"`
}

// mountResponse is the JSON representation of a volume mount.
type mountResponse struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// autoScaleResponse is the JSON representation of autoscaling config.
type autoScaleResponse struct {
	MinReplicas  int `json:"min_replicas"`
	MaxReplicas  int `json:"max_replicas"`
	CPUTarget    int `json:"cpu_target,omitempty"`
	MemoryTarget int `json:"memory_target,omitempty"`
}

// disruptionBudgetResponse is the JSON representation of a PDB.
type disruptionBudgetResponse struct {
	MinAvailable   int `json:"min_available,omitempty"`
	MaxUnavailable int `json:"max_unavailable,omitempty"`
}

// securityResponse is the JSON representation of container security settings.
type securityResponse struct {
	RunAsUser        int      `json:"run_as_user,omitempty"`
	RunAsGroup       int      `json:"run_as_group,omitempty"`
	ReadOnlyRoot     bool     `json:"read_only_root,omitempty"`
	DropCapabilities []string `json:"drop_capabilities,omitempty"`
	AddCapabilities  []string `json:"add_capabilities,omitempty"`
}

// permissionResponse is the JSON representation of an RBAC permission.
type permissionResponse struct {
	Verbs       []string `json:"verbs"`
	Resources   []string `json:"resources"`
	Namespace   string   `json:"namespace,omitempty"`
	ClusterWide bool     `json:"cluster_wide,omitempty"`
}

// schedulingResponse is the JSON representation of scheduling constraints.
type schedulingResponse struct {
	SpreadTopology string `json:"spread_topology,omitempty"`
	AntiAffinity   bool   `json:"anti_affinity,omitempty"`
}

// networkPolicyResponse is the JSON representation of network access controls.
type networkPolicyResponse struct {
	AllowFrom      []string `json:"allow_from,omitempty"`
	AllowNamespace []string `json:"allow_namespace,omitempty"`
	DenyAll        bool     `json:"deny_all,omitempty"`
}

// lifecycleResponse is the JSON representation of container lifecycle hooks.
type lifecycleResponse struct {
	PreStopCommand   string `json:"pre_stop_command,omitempty"`
	PreStopWait      int    `json:"pre_stop_wait,omitempty"`
	PostStartCommand string `json:"post_start_command,omitempty"`
}

// updateStrategyResponse is the JSON representation of rolling update config.
type updateStrategyResponse struct {
	MaxSurge       string `json:"max_surge,omitempty"`
	MaxUnavailable string `json:"max_unavailable,omitempty"`
}

// ingredientResponse is the JSON representation of a dependency edge.
type ingredientResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// slicesListResponse wraps the slice array for the slices list endpoint.
type slicesListResponse struct {
	Slices []sliceResponse `json:"slices"`
}

// ingredientsListResponse wraps the ingredients array for the ingredients endpoint.
type ingredientsListResponse struct {
	Ingredients []ingredientResponse `json:"ingredients"`
}

// handleHealth returns a handler that reports server health.
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}
}

// readyzResponse is the JSON structure returned by the readiness endpoint.
type readyzResponse struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks"`
}

// checkResult is the JSON structure for an individual readiness check.
type checkResult struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleReadyz returns a handler that checks backend dependencies and reports readiness.
func (s *Server) handleReadyz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		checks := make(map[string]checkResult)
		allOK := true

		// Database check.
		start := time.Now()
		if s.cfg.Store != nil {
			if err := s.cfg.Store.Ping(ctx); err != nil {
				checks["database"] = checkResult{Status: "error", Error: err.Error()}
				allOK = false
			} else {
				checks["database"] = checkResult{Status: "ok", LatencyMs: time.Since(start).Milliseconds()}
			}
		} else {
			checks["database"] = checkResult{Status: "error", Error: "store not configured"}
			allOK = false
		}

		status := "ready"
		code := http.StatusOK
		if !allOK {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}

		JSON(w, code, readyzResponse{Status: status, Checks: checks})
	}
}

// handleVersion returns a handler that reports build version metadata.
func (s *Server) handleVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, versionResponse{
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		})
	}
}

// handlePlan returns a handler that serves the full application plan.
func (s *Server) handlePlan() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := planResponse{
			Name:        s.cfg.Plan.Name,
			Slices:      buildSliceResponses(s),
			Ingredients: buildIngredientResponses(s),
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleSlices returns a handler that lists all slices.
func (s *Server) handleSlices() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, slicesListResponse{
			Slices: buildSliceResponses(s),
		})
	}
}

// handleSlice returns a handler that serves a single slice by name.
// It returns 404 if the slice is not found.
func (s *Server) handleSlice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		sl := s.cfg.Plan.SliceByName(name)
		if sl == nil {
			Error(w, http.StatusNotFound, "slice not found")
			return
		}

		JSON(w, http.StatusOK, toSliceResponse(*sl))
	}
}

// handleIngredients returns a handler that serves the dependency graph.
func (s *Server) handleIngredients() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, ingredientsListResponse{
			Ingredients: buildIngredientResponses(s),
		})
	}
}

// buildSliceResponses converts plan slices to response DTOs.
func buildSliceResponses(s *Server) []sliceResponse {
	slices := make([]sliceResponse, len(s.cfg.Plan.Slices))
	for i, sl := range s.cfg.Plan.Slices {
		slices[i] = toSliceResponse(sl)
	}
	return slices
}

// toSliceResponse converts a single plan.Slice to a sliceResponse.
func toSliceResponse(sl plan.Slice) sliceResponse {
	resp := sliceResponse{
		Name:             sl.Name,
		Kind:             string(sl.Kind),
		Image:            sl.Image,
		Port:             sl.Port,
		Public:           sl.Public,
		Replicas:         sl.Replicas,
		HealthPath:       sl.HealthPath,
		Env:              sl.Env,
		Schedule:         sl.Schedule,
		RunOnce:          sl.RunOnce,
		Parallelism:      sl.Parallelism,
		Retries:          sl.Retries,
		DaemonMode:       sl.DaemonMode,
		OrderedStartup:   sl.OrderedStartup,
		PeerDiscovery:    sl.PeerDiscovery,
		StatefulStorage:  sl.StatefulStorage,
		DNSName:          sl.DNSName,
		ServiceAccount:   sl.ServiceAccount,
		GracefulShutdown: sl.GracefulShutdown,
		RestartPolicy:    sl.RestartPolicy,
		Domain:           sl.Domain,
	}

	resp.Ports = toPortResponses(sl.Ports)
	resp.Probes = toProbeResponses(sl.Probes)
	resp.InitSteps = toInitStepResponses(sl.InitSteps)
	resp.Sidecars = toSidecarResponses(sl.Sidecars)
	resp.Mounts = toMountResponses(sl.Mounts)
	resp.Permissions = toPermissionResponses(sl.Permissions)

	if sl.AutoScale != nil {
		resp.AutoScale = &autoScaleResponse{
			MinReplicas:  sl.AutoScale.MinReplicas,
			MaxReplicas:  sl.AutoScale.MaxReplicas,
			CPUTarget:    sl.AutoScale.CPUTarget,
			MemoryTarget: sl.AutoScale.MemoryTarget,
		}
	}
	if sl.DisruptionBudget != nil {
		resp.DisruptionBudget = &disruptionBudgetResponse{
			MinAvailable:   sl.DisruptionBudget.MinAvailable,
			MaxUnavailable: sl.DisruptionBudget.MaxUnavailable,
		}
	}
	if sl.Security != nil {
		resp.Security = &securityResponse{
			RunAsUser:        sl.Security.RunAsUser,
			RunAsGroup:       sl.Security.RunAsGroup,
			ReadOnlyRoot:     sl.Security.ReadOnlyRoot,
			DropCapabilities: sl.Security.DropCapabilities,
			AddCapabilities:  sl.Security.AddCapabilities,
		}
	}
	if sl.Scheduling != nil {
		resp.Scheduling = &schedulingResponse{
			SpreadTopology: sl.Scheduling.SpreadTopology,
			AntiAffinity:   sl.Scheduling.AntiAffinity,
		}
	}
	if sl.NetworkPolicy != nil {
		resp.NetworkPolicy = &networkPolicyResponse{
			AllowFrom:      sl.NetworkPolicy.AllowFrom,
			AllowNamespace: sl.NetworkPolicy.AllowNamespace,
			DenyAll:        sl.NetworkPolicy.DenyAll,
		}
	}
	if sl.Lifecycle != nil {
		resp.Lifecycle = &lifecycleResponse{
			PreStopCommand:   sl.Lifecycle.PreStopCommand,
			PreStopWait:      sl.Lifecycle.PreStopWait,
			PostStartCommand: sl.Lifecycle.PostStartCommand,
		}
	}
	if sl.UpdateStrategy != nil {
		resp.UpdateStrategy = &updateStrategyResponse{
			MaxSurge:       sl.UpdateStrategy.MaxSurge,
			MaxUnavailable: sl.UpdateStrategy.MaxUnavailable,
		}
	}

	return resp
}

// toPortResponses converts plan port specs to response DTOs.
func toPortResponses(ports []plan.PortSpec) []portResponse {
	if len(ports) == 0 {
		return nil
	}
	out := make([]portResponse, len(ports))
	for i, p := range ports {
		out[i] = portResponse{Name: p.Name, Port: p.Port, Protocol: p.Protocol}
	}
	return out
}

// toProbeResponses converts plan probe specs to response DTOs.
func toProbeResponses(probes []plan.ProbeSpec) []probeResponse {
	if len(probes) == 0 {
		return nil
	}
	out := make([]probeResponse, len(probes))
	for i, p := range probes {
		out[i] = probeResponse{
			Type:     p.Type,
			HTTPPath: p.HTTPPath,
			Command:  p.Command,
			TCPPort:  p.TCPPort,
			Interval: p.Interval,
			Timeout:  p.Timeout,
			Delay:    p.Delay,
		}
	}
	return out
}

// toInitStepResponses converts plan init steps to response DTOs.
func toInitStepResponses(steps []plan.InitStep) []initStepResponse {
	if len(steps) == 0 {
		return nil
	}
	out := make([]initStepResponse, len(steps))
	for i, s := range steps {
		out[i] = initStepResponse{Image: s.Image, Command: s.Command}
	}
	return out
}

// toSidecarResponses converts plan sidecars to response DTOs.
func toSidecarResponses(sidecars []plan.Sidecar) []sidecarResponse {
	if len(sidecars) == 0 {
		return nil
	}
	out := make([]sidecarResponse, len(sidecars))
	for i, sc := range sidecars {
		out[i] = sidecarResponse{
			Name:  sc.Name,
			Image: sc.Image,
			Ports: toPortResponses(sc.Ports),
		}
	}
	return out
}

// toMountResponses converts plan mount specs to response DTOs.
func toMountResponses(mounts []plan.MountSpec) []mountResponse {
	if len(mounts) == 0 {
		return nil
	}
	out := make([]mountResponse, len(mounts))
	for i, m := range mounts {
		out[i] = mountResponse{
			Type:     m.Type,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		}
	}
	return out
}

// toPermissionResponses converts plan permissions to response DTOs.
func toPermissionResponses(perms []plan.Permission) []permissionResponse {
	if len(perms) == 0 {
		return nil
	}
	out := make([]permissionResponse, len(perms))
	for i, p := range perms {
		out[i] = permissionResponse{
			Verbs:       p.Verbs,
			Resources:   p.Resources,
			Namespace:   p.Namespace,
			ClusterWide: p.ClusterWide,
		}
	}
	return out
}

// buildIngredientResponses converts plan ingredients to response DTOs.
func buildIngredientResponses(s *Server) []ingredientResponse {
	ingredients := make([]ingredientResponse, len(s.cfg.Plan.Ingredients))
	for i, ing := range s.cfg.Plan.Ingredients {
		ingredients[i] = ingredientResponse{
			From: ing.From,
			To:   ing.To,
		}
	}
	return ingredients
}

// doctorResponse is the JSON structure returned by the doctor endpoint.
type doctorResponse struct {
	Findings []findingResponse     `json:"findings"`
	Summary  doctorSummaryResponse `json:"summary"`
}

// findingResponse is the JSON representation of a single diagnostic finding.
type findingResponse struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

// doctorSummaryResponse is the JSON representation of the doctor report summary.
type doctorSummaryResponse struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
	OK       int `json:"ok"`
}

// statusResponse is the JSON structure returned by the status endpoint.
type statusResponse struct {
	Containers []containerResponse `json:"containers"`
}

// containerResponse is the JSON representation of a running container.
type containerResponse struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	State   string `json:"state"`
	Image   string `json:"image"`
	Ports   string `json:"ports,omitempty"`
}

// handleDoctor returns a handler that runs the diagnostic engine and returns
// findings as JSON. Returns 503 if no doctor engine is configured.
// Concurrency is limited by the subprocess semaphore.
func (s *Server) handleDoctor() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Doctor == nil {
			Error(w, http.StatusServiceUnavailable, "doctor not configured")
			return
		}

		if !s.acquireSubprocess(w) {
			return
		}
		defer s.releaseSubprocess()

		report, err := s.cfg.Doctor.Run(r.Context(), s.cfg.Plan)
		if err != nil {
			Error(w, http.StatusInternalServerError, "doctor run failed")
			return
		}

		findings := make([]findingResponse, len(report.Findings))
		for i, f := range report.Findings {
			findings[i] = findingResponse{
				Rule:     f.Rule,
				Severity: string(f.Severity),
				Message:  f.Message,
				Fix:      f.Fix,
			}
		}

		JSON(w, http.StatusOK, doctorResponse{
			Findings: findings,
			Summary: doctorSummaryResponse{
				Errors:   report.Summary.Errors,
				Warnings: report.Summary.Warnings,
				Info:     report.Summary.Info,
				OK:       report.Summary.OK,
			},
		})
	}
}

// handleStatus returns a handler that queries Docker Compose for container
// status and returns the results as JSON.
// Concurrency is limited by the subprocess semaphore.
func (s *Server) handleStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.acquireSubprocess(w) {
			return
		}
		defer s.releaseSubprocess()

		q := &local.StatusQuery{ProjectDir: s.cfg.ProjectDir}

		containers, err := q.Run(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to query container status")
			return
		}

		resp := statusResponse{
			Containers: make([]containerResponse, len(containers)),
		}
		for i, c := range containers {
			resp.Containers[i] = containerResponse{
				Name:    c.Name,
				Service: c.Service,
				State:   c.State,
				Image:   c.Image,
				Ports:   c.Ports,
			}
		}

		JSON(w, http.StatusOK, resp)
	}
}

// acquireSubprocess tries to acquire the subprocess semaphore without blocking.
// Returns true if acquired, or writes a 429 error and returns false.
func (s *Server) acquireSubprocess(w http.ResponseWriter) bool {
	select {
	case s.subSem <- struct{}{}:
		return true
	default:
		Error(w, http.StatusTooManyRequests, "too many concurrent requests")
		return false
	}
}

// releaseSubprocess releases one slot in the subprocess semaphore.
func (s *Server) releaseSubprocess() {
	<-s.subSem
}
