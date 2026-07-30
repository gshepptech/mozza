package local

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/gshepptech/mozza/internal/plan"
)

// k8sCPUToCompose converts a Kubernetes CPU value (e.g., "500m", "1", "2")
// to a Docker Compose cpus float string (e.g., "0.5", "1", "2").
//
// Both converters here parse with resource.ParseQuantity rather than reading
// suffixes off the string, because the validator now admits every quantity
// Kubernetes does — "2G", "1e3", "500u" and "1Ei" among them — and a converter
// that recognises only the handful of suffixes an old regex allowed would pass
// the rest through verbatim into a file Compose then refuses to load.
//
// A value that will not parse is returned unchanged. That branch is unreachable
// for a plan that passed Validate and exists only because these compilers are
// also reachable with a hand-built plan.
func k8sCPUToCompose(cpu string) string {
	if cpu == "" {
		return ""
	}

	q, err := resource.ParseQuantity(cpu)
	if err != nil {
		return cpu
	}

	return strconv.FormatFloat(q.AsApproximateFloat64(), 'f', -1, 64)
}

// k8sMemToCompose converts a Kubernetes memory value (e.g., "256Mi", "1Gi")
// to a Docker Compose memory string (e.g., "256M", "1G").
//
// Compose's K/M/G are 1024-based and it rejects the "i" forms outright with
// "invalid suffix", so the conversion is load-bearing rather than cosmetic:
// "256M" is exactly 256Mi to Compose, which is why the byte count is preserved
// across the rename. Verified with `docker compose config`, not inferred.
//
// Formatting back into a unit rather than emitting a raw byte count keeps a file
// that users read and hand-edit legible. The largest exact unit wins, so a value
// that is a whole number of GiB stays one.
func k8sMemToCompose(mem string) string {
	if mem == "" {
		return ""
	}

	q, err := resource.ParseQuantity(mem)
	if err != nil {
		return mem
	}

	// The b > 0 guard does two jobs, and both matter. It keeps a zero limit as
	// "0" instead of "0G", and it routes every negative into the byte-count
	// branch — Compose rejects "-1G" and "-1073741824" alike, so neither form
	// ships a working file, but only the second reports the number the author
	// actually wrote. Relaxing this to b >= 0 silently turns 0 into "0G".
	b := q.Value()

	switch {
	case b > 0 && b%(1<<30) == 0:
		return strconv.FormatInt(b>>30, 10) + "G"
	case b > 0 && b%(1<<20) == 0:
		return strconv.FormatInt(b>>20, 10) + "M"
	case b > 0 && b%(1<<10) == 0:
		return strconv.FormatInt(b>>10, 10) + "K"
	default:
		return strconv.FormatInt(b, 10)
	}
}

// Healthcheck timing defaults. They apply only where the recipe states no
// timing of its own — a probe's own interval, timeout and delay win.
const (
	defaultHealthcheckInterval = "10s"
	defaultHealthcheckTimeout  = "5s"
	defaultHealthcheckRetries  = 3
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
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	StartPeriod string   `yaml:"start_period,omitempty"`
	Retries     int      `yaml:"retries"`
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
			return nil, fmt.Errorf("BuildComposeFileWithWarnings: slice %q: %w", s.Name, err)
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
	// addFileMounts is called from BuildComposeFileWithWarnings (needs composeFile access).
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

// addHealthcheck configures the one health check Compose allows per service.
func addHealthcheck(svc *service, s plan.Slice) {
	hc, _ := chooseHealthcheck(s)
	if hc == nil {
		return
	}

	svc.Healthcheck = hc
}

// chooseHealthcheck picks the probe that becomes the service's health check and
// builds it, returning the probe it came from so callers can name it.
//
// Emittability is decided inside the walk rather than after it: a slice whose
// readiness check is a TCP one and whose liveness check is an HTTP one still
// gets the health check the liveness probe describes, instead of nothing at all.
// Within a type the last declaration wins, matching internal/k8s, where each
// assignment overwrites the previous probe of that type. Returns a nil check
// when no candidate can be expressed.
func chooseHealthcheck(s plan.Slice) (*healthcheck, plan.ProbeSpec) {
	// Resolved the same way the Kubernetes backend resolves it, so a recipe's
	// health check cannot point at different ports on the two targets.
	port := s.PrimaryPort()

	// Backward compat: with no probes the legacy single path reads as a
	// readiness check — the same precedence internal/k8s's buildContainer uses.
	if len(s.Probes) == 0 {
		legacy := plan.ProbeSpec{Type: "readiness", HTTPPath: s.HealthPath}
		if hc := probeHealthcheck(legacy, port); hc != nil {
			return hc, legacy
		}
		return nil, plan.ProbeSpec{}
	}

	// Readiness first because Compose's health check is what
	// `depends_on: condition: service_healthy` gates on — that asks "is this
	// ready to serve", not "should this be restarted". Compose has no
	// restart-on-unhealthy at all, so preferring liveness would promise a
	// restart it never performs.
	for _, want := range []string{"readiness", "liveness", "startup"} {
		for i := len(s.Probes) - 1; i >= 0; i-- {
			ps := s.Probes[i]
			if !strings.EqualFold(ps.Type, want) {
				continue
			}
			if hc := probeHealthcheck(ps, port); hc != nil {
				return hc, ps
			}
		}
	}

	return nil, plan.ProbeSpec{}
}

// probeHealthcheck converts a plan.ProbeSpec into a Compose health check,
// mirroring internal/k8s's buildProbe field for field so one recipe cannot mean
// two different checks on the two targets: an HTTP path connects to the probe's
// own port or the slice's, and a command becomes the same argv Kubernetes execs.
//
// Returns nil for a check Compose cannot run. A TCP check is one: the Docker
// daemon runs a command inside the container and never opens a socket. So is a
// check with nothing to check, or an HTTP check with no port to reach — the
// cases buildProbe declines for the same reason, and plan.Validate rejects.
func probeHealthcheck(ps plan.ProbeSpec, slicePort int) *healthcheck {
	var test []string

	switch {
	case ps.HTTPPath != "":
		httpPort := ps.TCPPort
		if httpPort == 0 {
			httpPort = slicePort
		}
		if httpPort == 0 {
			return nil
		}
		test = []string{"CMD", "curl", "-f", fmt.Sprintf("http://localhost:%d%s", httpPort, ps.HTTPPath)}
	case ps.TCPPort > 0:
		return nil
	case ps.Command != "":
		test = append([]string{"CMD"}, strings.Fields(ps.Command)...)
	default:
		return nil
	}

	hc := &healthcheck{
		Test:     test,
		Interval: defaultHealthcheckInterval,
		Timeout:  defaultHealthcheckTimeout,
		Retries:  defaultHealthcheckRetries,
	}

	if ps.Interval > 0 {
		hc.Interval = fmt.Sprintf("%ds", ps.Interval)
	}
	if ps.Timeout > 0 {
		hc.Timeout = fmt.Sprintf("%ds", ps.Timeout)
	}
	// start_period is the closest Compose has to an initial delay, not the same
	// thing: Kubernetes suppresses probing for the window, Compose probes
	// throughout it but declines to count the failures toward retries.
	if ps.Delay > 0 {
		hc.StartPeriod = fmt.Sprintf("%ds", ps.Delay)
	}

	return hc
}

// describeProbe names a probe the way the recipe directive that declared it
// reads — "readiness check on /healthz". The parser always sets Type, but a
// hand-built ProbeSpec reaching the exported compiler need not, so the fallback
// still reads as English, following probeSubject in internal/plan.
func describeProbe(ps plan.ProbeSpec) string {
	kind := "health check"
	if ps.Type != "" {
		kind = strings.ToLower(ps.Type) + " check"
	}

	switch {
	case ps.HTTPPath != "":
		return kind + " on " + ps.HTTPPath
	case ps.TCPPort > 0:
		return fmt.Sprintf("%s on TCP port %d", kind, ps.TCPPort)
	case ps.Command != "":
		return fmt.Sprintf("%s running %q", kind, ps.Command)
	}

	return kind
}

// sameHealthcheck reports whether two health checks would do the same thing.
func sameHealthcheck(a, b *healthcheck) bool {
	if a == nil || b == nil {
		return a == b
	}

	return slices.Equal(a.Test, b.Test) &&
		a.Interval == b.Interval &&
		a.Timeout == b.Timeout &&
		a.StartPeriod == b.StartPeriod &&
		a.Retries == b.Retries
}

// probeWarnings names every check on the slice that the local target could not
// carry. Silence here is documented as meaning nothing was lost, so a check that
// went missing has to be said out loud rather than left for the author to notice.
//
// A probe that is not the emitted check stays quiet when the check it would have
// produced is identical: "health check /x" expands into a readiness and a
// liveness probe on the same path, and warning about the twin would fire on
// almost every recipe in the catalog. That also suppresses the chosen probe's
// own entry, since it is trivially identical to itself.
func probeWarnings(s plan.Slice) []string {
	if len(s.Probes) == 0 {
		return nil
	}

	chosen, chosenProbe := chooseHealthcheck(s)

	var warnings []string

	// Stated as a consequence only. "Because the slice declares no readiness
	// check" would be false for a slice whose readiness check is a TCP one.
	if kind := strings.ToLower(chosenProbe.Type); kind == "liveness" || kind == "startup" {
		warnings = append(warnings, fmt.Sprintf(
			"%s: The %s became the Compose health check. Compose reports the container unhealthy but will not restart it the way Kubernetes does.",
			s.Name, describeProbe(chosenProbe)))
	}

	for _, ps := range s.Probes {
		hc := probeHealthcheck(ps, s.PrimaryPort())
		switch {
		case hc == nil && ps.TCPPort > 0:
			warnings = append(warnings, fmt.Sprintf(
				"%s: Compose has no TCP health check, so the %s was dropped. Give the check an HTTP path or a command to have Compose run it.",
				s.Name, describeProbe(ps)))
		case hc != nil && chosen == nil:
			// Compose could have run this one; chooseHealthcheck never offered it
			// the chance, because only the three known kinds are candidates.
			// Blaming the one-check-per-service limit here would be false — the
			// service has no health check at all.
			warnings = append(warnings, fmt.Sprintf(
				"%s: The %s is not a readiness, liveness or startup check, so it was dropped and nothing is checked locally. Give the check one of those three kinds to have Compose run it.",
				s.Name, describeProbe(ps)))
		case hc != nil && !sameHealthcheck(hc, chosen):
			warnings = append(warnings, fmt.Sprintf(
				"%s: Compose runs one health check per service, so the %s was dropped. Only the %s is checked locally.",
				s.Name, describeProbe(ps), describeProbe(chosenProbe)))
		}
	}

	return warnings
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
		// Deliberately does not promise the minimum replica count: addDeploy
		// substitutes MinReplicas only when the slice declares one replica or
		// fewer, so a slice with "run 5 copies" keeps 5. What is true either way
		// is that the count no longer moves.
		warnings = append(warnings,
			fmt.Sprintf("%s: Auto-scaling is Kubernetes-only. Running a fixed replica count locally.", s.Name))
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
			fmt.Sprintf("%s: Scheduled tasks are not supported in Docker Compose. Service will run once on startup.", s.Name))
	}

	warnings = append(warnings, probeWarnings(s)...)

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
