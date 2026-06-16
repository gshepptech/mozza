package local

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/plan"
)

// k8sCPUToCompose converts a Kubernetes CPU value (e.g., "500m", "1", "2")
// to a Docker Compose cpus float string (e.g., "0.5", "1", "2").
func k8sCPUToCompose(cpu string) string {
	if cpu == "" {
		return ""
	}
	if strings.HasSuffix(cpu, "m") {
		millis, err := strconv.ParseFloat(strings.TrimSuffix(cpu, "m"), 64)
		if err != nil {
			return cpu
		}
		return strconv.FormatFloat(millis/1000, 'f', -1, 64)
	}
	return cpu // already a plain number
}

// k8sMemToCompose converts a Kubernetes memory value (e.g., "256Mi", "1Gi")
// to a Docker Compose memory string (e.g., "256M", "1G").
func k8sMemToCompose(mem string) string {
	if mem == "" {
		return ""
	}
	// Docker Compose uses M/G (powers of 1000) but also accepts them as
	// binary-compatible. K8s Mi/Gi are binary. Close enough for limits.
	mem = strings.Replace(mem, "Mi", "M", 1)
	mem = strings.Replace(mem, "Gi", "G", 1)
	mem = strings.Replace(mem, "Ki", "K", 1)
	return mem
}

// Healthcheck timing constants.
const (
	healthcheckInterval = "10s"
	healthcheckTimeout  = "5s"
	healthcheckRetries  = 3
)

// networkSuffix is appended to the app name to form the Docker network name.
const networkSuffix = "-net"

// volumeSuffix is appended to the slice name to form the Docker volume name.
const volumeSuffix = "-data"

// composeFile represents a docker-compose.yml file.
type composeFile struct {
	Services map[string]service `yaml:"services"`
	Volumes  map[string]volume  `yaml:"volumes,omitempty"`
	Networks map[string]network `yaml:"networks,omitempty"`
}

// service represents a single service in a Docker Compose file.
type service struct {
	Image           string            `yaml:"image"`
	Command         string            `yaml:"command,omitempty"`
	User            string            `yaml:"user,omitempty"`
	ReadOnly        bool              `yaml:"read_only,omitempty"`
	Ports           []string          `yaml:"ports,omitempty"`
	Expose          []string          `yaml:"expose,omitempty"`
	Environment     map[string]string `yaml:"environment,omitempty"`
	DependsOn       interface{}       `yaml:"depends_on,omitempty"`
	Volumes         []string          `yaml:"volumes,omitempty"`
	Networks        []string          `yaml:"networks,omitempty"`
	NetworkMode     string            `yaml:"network_mode,omitempty"`
	Restart         string            `yaml:"restart,omitempty"`
	Deploy          *deploy           `yaml:"deploy,omitempty"`
	Healthcheck     *healthcheck      `yaml:"healthcheck,omitempty"`
	Labels          map[string]string `yaml:"labels,omitempty"`
	CapDrop         []string          `yaml:"cap_drop,omitempty"`
	CapAdd          []string          `yaml:"cap_add,omitempty"`
	StopGracePeriod string            `yaml:"stop_grace_period,omitempty"`
}

// dependsOnCondition is used for structured depends_on entries with conditions.
type dependsOnCondition struct {
	Condition string `yaml:"condition"`
}

// deploy holds deployment configuration for a service.
type deploy struct {
	Replicas  int              `yaml:"replicas,omitempty"`
	Resources *deployResources `yaml:"resources,omitempty"`
}

// deployResources holds resource limits for a compose service.
type deployResources struct {
	Limits resourceLimits `yaml:"limits"`
}

// resourceLimits specifies CPU and memory limits.
type resourceLimits struct {
	CPUs   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// healthcheck configures a container health check.
type healthcheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}

// volume represents a named Docker volume.
type volume struct{}

// network represents a Docker network.
type network struct {
	Driver string `yaml:"driver"`
}

// buildResult holds the compose file and any warnings generated during building.
type buildResult struct {
	File     *composeFile
	Warnings []string
}

// BuildComposeFile converts an AppPlan into a composeFile structure.
func BuildComposeFile(p *plan.AppPlan) (*composeFile, error) {
	br, err := BuildComposeFileWithWarnings(p)
	if err != nil {
		return nil, err
	}
	return br.File, nil
}

// BuildComposeFileWithWarnings converts an AppPlan into a composeFile plus warnings.
func BuildComposeFileWithWarnings(p *plan.AppPlan) (*buildResult, error) {
	cf := &composeFile{
		Services: make(map[string]service),
		Volumes:  make(map[string]volume),
		Networks: make(map[string]network),
	}

	var warnings []string

	netName := p.Name + networkSuffix
	cf.Networks[netName] = network{Driver: "bridge"}

	for _, s := range p.Slices {
		svc, err := buildService(s, netName)
		if err != nil {
			return nil, fmt.Errorf("BuildComposeFile: slice %q: %w", s.Name, err)
		}
		addFileMounts(cf, &svc, s)
		cf.Services[s.Name] = svc
		addVolumes(cf, s)

		// Generate init container services.
		addInitServices(cf, s, netName)

		// Generate sidecar services.
		addSidecarServices(cf, s, netName)

		// Add stateful volumes.
		addStatefulVolumes(cf, s)

		// Collect K8s-only warnings.
		warnings = append(warnings, collectWarnings(s)...)
	}

	return &buildResult{File: cf, Warnings: warnings}, nil
}

// buildService converts a single Slice into a compose service definition.
func buildService(s plan.Slice, netName string) (service, error) {
	svc := service{
		Image:    s.Image,
		Networks: []string{netName},
	}

	if len(s.Needs) > 0 {
		svc.DependsOn = s.Needs
	}

	if len(s.Env) > 0 {
		svc.Environment = sortedEnv(s.Env)
	}

	// Inject default env vars for database/cache images so they start
	// without manual configuration.
	addDatabaseEnv(&svc, s)

	// Apply kind-specific restart policy defaults.
	applyKindDefaults(&svc, s)

	if s.RestartPolicy != "" {
		svc.Restart = s.RestartPolicy
	}

	addPorts(&svc, s)
	addHealthcheck(&svc, s)
	addDeploy(&svc, s)
	addVolumeMount(&svc, s)
	// addFileMounts is called from BuildComposeFile (needs composeFile access).
	addSecurityContext(&svc, s)
	addGracefulShutdown(&svc, s)
	addInitDependsOn(&svc, s)

	return svc, nil
}

// applyKindDefaults sets kind-specific defaults for restart policy and labels.
func applyKindDefaults(svc *service, s plan.Slice) {
	switch s.Kind {
	case plan.SliceKindTask:
		if s.RestartPolicy == "" {
			svc.Restart = "no"
		}
	case plan.SliceKindScheduled:
		if s.RestartPolicy == "" {
			svc.Restart = "no"
		}
		if s.Schedule != "" {
			if svc.Labels == nil {
				svc.Labels = make(map[string]string)
			}
			svc.Labels["mozza.schedule"] = s.Schedule
		}
	case plan.SliceKindDaemon:
		if svc.Labels == nil {
			svc.Labels = make(map[string]string)
		}
		svc.Labels["mozza.daemon-mode"] = "true"
	case plan.SliceKindWeb, plan.SliceKindWorker, plan.SliceKindDatabase,
		plan.SliceKindCache, plan.SliceKindAPI, plan.SliceKindStateful,
		plan.SliceKindGateway:
		// No kind-specific defaults for these.
	}
}

// sortedEnv returns a copy of the env map for deterministic output.
func sortedEnv(env map[string]string) map[string]string {
	result := make(map[string]string, len(env))
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result[k] = env[k]
	}
	return result
}

// addPorts configures port mappings based on slice kind and visibility.
// When replicas > 1, host port bindings are replaced with expose-only
// to avoid port conflicts between container instances.
func addPorts(svc *service, s plan.Slice) {
	// Handle multi-port via Ports[] first.
	if len(s.Ports) > 0 {
		addMultiPorts(svc, s)
		return
	}

	if s.Port == 0 {
		return
	}

	// With multiple replicas, only one container can bind the host port.
	// Use expose instead so all replicas are reachable via the network.
	if s.Replicas > 1 {
		svc.Expose = []string{fmt.Sprintf("%d", s.Port)}
		return
	}

	switch s.Kind {
	case plan.SliceKindWeb, plan.SliceKindGateway:
		if s.Public {
			svc.Ports = []string{fmt.Sprintf("%d:%d", s.Port, s.Port)}
		}
	case plan.SliceKindAPI:
		// API services use expose (internal ports) by default.
		svc.Expose = []string{fmt.Sprintf("%d", s.Port)}
	case plan.SliceKindDatabase, plan.SliceKindCache:
		svc.Ports = []string{fmt.Sprintf("%d:%d", s.Port, s.Port)}
	case plan.SliceKindWorker, plan.SliceKindTask, plan.SliceKindScheduled,
		plan.SliceKindDaemon:
		// These kinds do not expose ports.
	case plan.SliceKindStateful:
		// Stateful services expose ports for peer discovery.
		svc.Ports = []string{fmt.Sprintf("%d:%d", s.Port, s.Port)}
	}
}

// addMultiPorts adds multiple port mappings from the Ports slice.
func addMultiPorts(svc *service, s plan.Slice) {
	switch s.Kind {
	case plan.SliceKindAPI:
		for _, p := range s.Ports {
			svc.Expose = append(svc.Expose, fmt.Sprintf("%d", p.Port))
		}
	case plan.SliceKindWorker, plan.SliceKindTask, plan.SliceKindScheduled:
		// No ports for these kinds.
	case plan.SliceKindWeb, plan.SliceKindDatabase, plan.SliceKindCache,
		plan.SliceKindStateful, plan.SliceKindGateway, plan.SliceKindDaemon:
		for _, p := range s.Ports {
			svc.Ports = append(svc.Ports, fmt.Sprintf("%d:%d", p.Port, p.Port))
		}
	}
}

// addHealthcheck configures a health check if the slice has a HealthPath.
func addHealthcheck(svc *service, s plan.Slice) {
	if s.HealthPath == "" || s.Port == 0 {
		return
	}

	svc.Healthcheck = &healthcheck{
		Test:     []string{"CMD", "curl", "-f", fmt.Sprintf("http://localhost:%d%s", s.Port, s.HealthPath)},
		Interval: healthcheckInterval,
		Timeout:  healthcheckTimeout,
		Retries:  healthcheckRetries,
	}
}

// addDeploy configures deployment replicas and resource limits.
func addDeploy(svc *service, s plan.Slice) {
	replicas := s.Replicas

	// For auto-scaled slices, use MinReplicas as the local replica count.
	if s.AutoScale != nil && s.AutoScale.MinReplicas > 0 && replicas <= 1 {
		replicas = s.AutoScale.MinReplicas
	}

	needsDeploy := replicas > 1 || s.Resources != nil

	if !needsDeploy {
		return
	}

	d := &deploy{}

	if replicas > 1 {
		d.Replicas = replicas
	}

	if s.Resources != nil && (s.Resources.CPULimit != "" || s.Resources.MemoryLimit != "") {
		d.Resources = &deployResources{
			Limits: resourceLimits{
				CPUs:   k8sCPUToCompose(s.Resources.CPULimit),
				Memory: k8sMemToCompose(s.Resources.MemoryLimit),
			},
		}
	}

	svc.Deploy = d
}

// addVolumeMount attaches a named volume mount to database and cache services.
func addVolumeMount(svc *service, s plan.Slice) {
	volName := s.Name + volumeSuffix

	switch s.Kind {
	case plan.SliceKindDatabase:
		if s.Database != nil && s.Database.Storage != "" {
			mountPath := s.Database.MountPath
			if mountPath == "" {
				mountPath = "/var/lib/data"
			}
			svc.Volumes = append(svc.Volumes, volName+":"+mountPath)
		}
	case plan.SliceKindCache:
		if s.Cache != nil && s.Cache.Storage != "" {
			mountPath := s.Cache.MountPath
			if mountPath == "" {
				mountPath = "/data"
			}
			svc.Volumes = append(svc.Volumes, volName+":"+mountPath)
		}
	case plan.SliceKindStateful:
		if s.StatefulStorage != "" {
			mountPath := "/data"
			svc.Volumes = append(svc.Volumes, volName+":"+mountPath)
		}
	case plan.SliceKindWeb, plan.SliceKindWorker, plan.SliceKindAPI,
		plan.SliceKindTask, plan.SliceKindScheduled, plan.SliceKindGateway,
		plan.SliceKindDaemon:
		// No volume mounts for these kinds.
	}
}

// addFileMounts adds bind mount volumes from the slice's Mounts field.
// Named volumes (source without "/" or "." prefix) are registered in the
// top-level volumes section of the compose file.
func addFileMounts(cf *composeFile, svc *service, s plan.Slice) {
	for _, m := range s.Mounts {
		mount := m.Source + ":" + m.Target
		if m.ReadOnly {
			mount += ":ro"
		}
		svc.Volumes = append(svc.Volumes, mount)

		// If the source is a named volume (not a host path), register it.
		if !strings.HasPrefix(m.Source, "/") && !strings.HasPrefix(m.Source, ".") {
			cf.Volumes[m.Source] = volume{}
		}
	}
}

// addDatabaseEnv injects required env vars for well-known database and cache
// images so they start without manual configuration in local dev.
func addDatabaseEnv(svc *service, s plan.Slice) {
	if s.Kind != plan.SliceKindDatabase && s.Kind != plan.SliceKindCache {
		return
	}

	// Only inject if the user hasn't already set these.
	has := func(key string) bool {
		if svc.Environment == nil {
			return false
		}
		_, ok := svc.Environment[key]
		return ok
	}

	img := s.Image
	switch {
	case strings.Contains(img, "postgres"):
		if !has("POSTGRES_PASSWORD") {
			if svc.Environment == nil {
				svc.Environment = make(map[string]string)
			}
			svc.Environment["POSTGRES_PASSWORD"] = "mozza"
			svc.Environment["POSTGRES_DB"] = s.Name
		}
	case strings.Contains(img, "mysql") || strings.Contains(img, "mariadb"):
		if !has("MYSQL_ROOT_PASSWORD") {
			if svc.Environment == nil {
				svc.Environment = make(map[string]string)
			}
			svc.Environment["MYSQL_ROOT_PASSWORD"] = "mozza"
			svc.Environment["MYSQL_DATABASE"] = s.Name
		}
	case strings.Contains(img, "mongo"):
		if !has("MONGO_INITDB_ROOT_USERNAME") {
			if svc.Environment == nil {
				svc.Environment = make(map[string]string)
			}
			svc.Environment["MONGO_INITDB_ROOT_USERNAME"] = "mozza"
			svc.Environment["MONGO_INITDB_ROOT_PASSWORD"] = "mozza"
		}
	}
}

// addSecurityContext maps security settings to compose service fields.
func addSecurityContext(svc *service, s plan.Slice) {
	if s.Security == nil {
		return
	}

	if s.Security.RunAsUser > 0 {
		svc.User = fmt.Sprintf("%d", s.Security.RunAsUser)
	}

	if s.Security.ReadOnlyRoot {
		svc.ReadOnly = true
	}

	if len(s.Security.DropCapabilities) > 0 {
		svc.CapDrop = s.Security.DropCapabilities
	}

	if len(s.Security.AddCapabilities) > 0 {
		svc.CapAdd = s.Security.AddCapabilities
	}
}

// addGracefulShutdown sets stop_grace_period from GracefulShutdown seconds.
func addGracefulShutdown(svc *service, s plan.Slice) {
	if s.GracefulShutdown > 0 {
		svc.StopGracePeriod = fmt.Sprintf("%ds", s.GracefulShutdown)
	}
}

// addInitDependsOn modifies depends_on to use structured format when init steps exist.
func addInitDependsOn(svc *service, s plan.Slice) {
	if len(s.InitSteps) == 0 {
		return
	}

	deps := make(map[string]dependsOnCondition)

	// Add init service dependencies with completion condition.
	for i := range s.InitSteps {
		initName := fmt.Sprintf("%s-init-%d", s.Name, i)
		deps[initName] = dependsOnCondition{Condition: "service_completed_successfully"}
	}

	// Preserve existing plain dependencies.
	if existing, ok := svc.DependsOn.([]string); ok {
		for _, dep := range existing {
			deps[dep] = dependsOnCondition{Condition: "service_started"}
		}
	}

	svc.DependsOn = deps
}

// addInitServices creates separate init services for each InitStep.
func addInitServices(cf *composeFile, s plan.Slice, netName string) {
	for i, init := range s.InitSteps {
		initName := fmt.Sprintf("%s-init-%d", s.Name, i)
		initSvc := service{
			Image:    init.Image,
			Restart:  "no",
			Networks: []string{netName},
		}

		if init.Command != "" {
			initSvc.Command = init.Command
		}

		if len(init.Env) > 0 {
			initSvc.Environment = sortedEnv(init.Env)
		}

		// Init containers depend on the same Needs as the parent.
		if len(s.Needs) > 0 {
			deps := make(map[string]dependsOnCondition)
			for _, need := range s.Needs {
				deps[need] = dependsOnCondition{Condition: "service_healthy"}
			}
			initSvc.DependsOn = deps
		}

		cf.Services[initName] = initSvc
	}
}

// addSidecarServices creates separate sidecar services with network_mode.
func addSidecarServices(cf *composeFile, s plan.Slice, netName string) {
	for _, sc := range s.Sidecars {
		sidecarName := s.Name + "-" + sc.Name
		sidecarSvc := service{
			Image:       sc.Image,
			NetworkMode: fmt.Sprintf("service:%s", s.Name),
			DependsOn:   []string{s.Name},
		}

		if len(sc.Env) > 0 {
			sidecarSvc.Environment = sortedEnv(sc.Env)
		}

		// Sidecar ports are exposed on the parent's network.
		for _, p := range sc.Ports {
			sidecarSvc.Ports = append(sidecarSvc.Ports, fmt.Sprintf("%d:%d", p.Port, p.Port))
		}

		cf.Services[sidecarName] = sidecarSvc
	}
}

// addStatefulVolumes registers named volumes for stateful slices.
func addStatefulVolumes(cf *composeFile, s plan.Slice) {
	if s.Kind == plan.SliceKindStateful && s.StatefulStorage != "" {
		volName := s.Name + volumeSuffix
		cf.Volumes[volName] = volume{}
	}
}

// addVolumes registers named volumes in the compose file for storage slices.
func addVolumes(cf *composeFile, s plan.Slice) {
	volName := s.Name + volumeSuffix

	switch s.Kind {
	case plan.SliceKindDatabase:
		if s.Database != nil && s.Database.Storage != "" {
			cf.Volumes[volName] = volume{}
		}
	case plan.SliceKindCache:
		if s.Cache != nil && s.Cache.Storage != "" {
			cf.Volumes[volName] = volume{}
		}
	case plan.SliceKindWeb, plan.SliceKindWorker, plan.SliceKindAPI,
		plan.SliceKindTask, plan.SliceKindScheduled, plan.SliceKindStateful,
		plan.SliceKindGateway, plan.SliceKindDaemon:
		// No named volumes for these kinds.
	}
}

// collectWarnings generates warnings for K8s-only features used by a slice.
func collectWarnings(s plan.Slice) []string {
	var warnings []string

	if len(s.Permissions) > 0 {
		warnings = append(warnings,
			fmt.Sprintf("%s: Permissions are Kubernetes-only. Skipped for local target.", s.Name))
	}

	if s.NetworkPolicy != nil {
		warnings = append(warnings,
			fmt.Sprintf("%s: Network policies are Kubernetes-only. All services can communicate locally.", s.Name))
	}

	if s.Scheduling != nil {
		warnings = append(warnings,
			fmt.Sprintf("%s: Node scheduling is Kubernetes-only. Skipped for local target.", s.Name))
	}

	if s.DaemonMode {
		warnings = append(warnings,
			fmt.Sprintf("%s: DaemonSet mode requires Docker Swarm. Running as single instance.", s.Name))
	}

	if s.PeerDiscovery {
		warnings = append(warnings,
			fmt.Sprintf("%s: Headless services are Kubernetes-only. Using standard DNS resolution.", s.Name))
	}

	if s.AutoScale != nil {
		warnings = append(warnings,
			fmt.Sprintf("%s: Auto-scaling is Kubernetes-only. Running with minimum replica count.", s.Name))
	}

	if s.DisruptionBudget != nil {
		warnings = append(warnings,
			fmt.Sprintf("%s: Disruption budgets are Kubernetes-only. Skipped for local target.", s.Name))
	}

	if s.UpdateStrategy != nil {
		warnings = append(warnings,
			fmt.Sprintf("%s: Update strategy is Kubernetes-only. Skipped for local target.", s.Name))
	}

	if s.Kind == plan.SliceKindScheduled {
		warnings = append(warnings,
			fmt.Sprintf("%s: Scheduled tasks not supported in Docker Compose. Service will run once on startup.", s.Name))
	}

	return warnings
}

// MarshalComposeFile serializes a composeFile to YAML bytes.
func MarshalComposeFile(cf *composeFile) ([]byte, error) {
	out, err := yaml.Marshal(cf)
	if err != nil {
		return nil, fmt.Errorf("marshalComposeFile: %w", err)
	}
	return out, nil
}

// buildSummary creates a human-readable summary of the generated compose file.
func buildSummary(p *plan.AppPlan, cf *composeFile) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Generated docker-compose.yml for %q with %d service(s)", p.Name, len(cf.Services))

	if len(cf.Volumes) > 0 {
		fmt.Fprintf(&b, ", %d volume(s)", len(cf.Volumes))
	}

	if len(cf.Networks) > 0 {
		fmt.Fprintf(&b, ", %d network(s)", len(cf.Networks))
	}

	b.WriteString(".")
	return b.String()
}
