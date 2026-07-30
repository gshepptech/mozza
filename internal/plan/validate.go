package plan

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// validSliceName matches DNS-compatible names: lowercase alphanumeric with
// optional hyphens, starting with a letter and ending with a letter or digit.
var validSliceName = regexp.MustCompile(`^[a-z]([a-z0-9-]*[a-z0-9])?$`)

// validHealthPath matches safe URL paths: must start with / and contain only
// alphanumeric, hyphens, underscores, dots, and slashes.
var validHealthPath = regexp.MustCompile(`^/[a-zA-Z0-9/_.-]*$`)

// validDomain matches simple domain names.
var validDomain = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

// validRestartPolicies lists allowed restart policy values.
var validRestartPolicies = map[string]bool{ //nolint:gochecknoglobals // immutable lookup table
	"always":         true,
	"unless-stopped": true,
	"on-failure":     true,
	"no":             true,
}

// validCronFields matches a cron expression with exactly 5 space-separated fields.
var validCronFields = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+\s+\S+$`)

// kindsWithReplicas are kinds that support replica count scaling.
var kindsWithReplicas = map[SliceKind]bool{ //nolint:gochecknoglobals // immutable lookup table
	SliceKindWeb:      true,
	SliceKindAPI:      true,
	SliceKindWorker:   true,
	SliceKindStateful: true,
	SliceKindGateway:  true,
}

// validResourceValue matches Kubernetes-style resource values (e.g., "500m", "256Mi", "1Gi", "0.5").
var validResourceValue = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(m|Mi|Gi|Ki|Ti)?$`)

// Validate checks an AppPlan for structural and semantic correctness.
// It returns a joined error containing all validation failures found.
// A nil return indicates the plan is valid.
func Validate(p *AppPlan) error {
	var errs []error

	errs = append(errs, validatePlanFields(p)...)
	errs = append(errs, validateSliceFields(p.Slices)...)
	errs = append(errs, validateDependencies(p.Slices)...)

	return errors.Join(errs...)
}

// validatePlanFields checks top-level plan constraints.
func validatePlanFields(p *AppPlan) []error {
	var errs []error

	if p.Name == "" {
		errs = append(errs, errors.New("plan name must not be empty"))
	}

	if len(p.Slices) == 0 {
		errs = append(errs, errors.New("plan must have at least one slice"))
	}

	return errs
}

// validateSliceFields checks individual slice fields and uniqueness.
func validateSliceFields(slices []Slice) []error {
	var errs []error

	seen := make(map[string]bool, len(slices))

	// Build a set of all slice names for cross-references (e.g. NetworkPolicy).
	allNames := make(map[string]bool, len(slices))
	for _, s := range slices {
		if s.Name != "" {
			allNames[s.Name] = true
		}
	}

	for i, s := range slices {
		errs = append(errs, validateOneSlice(i, s, seen, allNames)...)
	}

	return errs
}

// validateOneSlice checks a single slice for field-level validity.
func validateOneSlice(idx int, s Slice, seen, allNames map[string]bool) []error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, fmt.Errorf("slice[%d]: name must not be empty", idx))
	} else if seen[s.Name] {
		errs = append(errs, fmt.Errorf("slice[%d]: duplicate name %q", idx, s.Name))
	} else {
		seen[s.Name] = true
	}

	if s.Name != "" && (len(s.Name) > 63 || !validSliceName.MatchString(s.Name)) {
		errs = append(errs, fmt.Errorf("slice[%d] %q: name must be DNS-compatible (lowercase alphanumeric and hyphens, max 63 chars)", idx, s.Name))
	}

	if s.Image == "" {
		errs = append(errs, fmt.Errorf("slice[%d] %q: image must not be empty", idx, s.Name))
	} else if strings.ContainsAny(s.Image, " \t\n\r") {
		errs = append(errs, fmt.Errorf("slice[%d] %q: image must not contain whitespace", idx, s.Name))
	}

	if s.Port < 0 || s.Port > 65535 {
		errs = append(errs, fmt.Errorf("slice[%d] %q: port must be 0-65535, got %d", idx, s.Name, s.Port))
	}

	if s.Replicas < 0 {
		errs = append(errs, fmt.Errorf("slice[%d] %q: replicas must be >= 0, got %d", idx, s.Name, s.Replicas))
	}

	if s.HealthPath != "" && !validHealthPath.MatchString(s.HealthPath) {
		errs = append(errs, fmt.Errorf("slice[%d] %q: health path must start with / and contain only alphanumeric, hyphens, underscores, dots, and slashes", idx, s.Name))
	}

	errs = append(errs, validateProbeTargets(idx, s)...)

	for key := range s.Env {
		if problem := envKeyProblem(key); problem != "" {
			errs = append(errs, fmt.Errorf("slice[%d] %q: env key %q %s", idx, s.Name, key, problem))
		}
	}

	if s.Domain != "" && !validDomain.MatchString(s.Domain) {
		errs = append(errs, fmt.Errorf("slice[%d] %q: domain %q is not a valid domain name", idx, s.Name, s.Domain))
	}

	if s.RestartPolicy != "" && !validRestartPolicies[s.RestartPolicy] {
		errs = append(errs, fmt.Errorf("slice[%d] %q: restart policy %q must be one of: always, unless-stopped, on-failure, no", idx, s.Name, s.RestartPolicy))
	}

	if s.Resources != nil {
		if s.Resources.CPULimit != "" && !validResourceValue.MatchString(s.Resources.CPULimit) {
			errs = append(errs, fmt.Errorf("slice[%d] %q: cpu limit %q is not a valid resource value", idx, s.Name, s.Resources.CPULimit))
		}
		if s.Resources.MemoryLimit != "" && !validResourceValue.MatchString(s.Resources.MemoryLimit) {
			errs = append(errs, fmt.Errorf("slice[%d] %q: memory limit %q is not a valid resource value", idx, s.Name, s.Resources.MemoryLimit))
		}
	}

	// Kind-specific validation rules.
	errs = append(errs, validateKindConstraints(idx, s, allNames)...)

	return errs
}

// validateProbeTargets checks that every health check on a slice can actually be
// aimed: an HTTP check needs a port to connect to, and any check needs something
// to check. Compilers would otherwise have to invent a target or drop the check,
// and both hide the mistake until the rollout hangs.
//
// One "health check <path>" directive lands in three places — HealthPath, plus a
// readiness and a liveness ProbeSpec carrying the same path — so the no-port
// complaint is collected and reported once per slice rather than once per copy.
func validateProbeTargets(idx int, s Slice) []error {
	var errs []error

	// The path of the first check that resolves to no port, or "" when every
	// check has a target.
	unaimed := ""
	if s.HealthPath != "" && s.PrimaryPort() == 0 {
		unaimed = s.HealthPath
	}

	for _, ps := range s.Probes {
		switch {
		case ps.HTTPPath != "":
			if unaimed == "" && ps.TCPPort == 0 && s.PrimaryPort() == 0 {
				unaimed = ps.HTTPPath
			}
		case ps.TCPPort > 0 || ps.Command != "":
			// The check names its own target, so there is nothing to resolve.
		default:
			// The parser leaves Type set, but a hand-built ProbeSpec reaching the
			// exported compilers need not, so the message still names a subject.
			subject := "this probe"
			if ps.Type != "" {
				subject = ps.Type + " check"
			}
			errs = append(errs, fmt.Errorf("slice[%d] %q: %s must say what to check — give it an HTTP path, a TCP port, or a command", idx, s.Name, subject))
		}
	}

	if unaimed != "" {
		errs = append(errs, fmt.Errorf("slice[%d] %q: health check %q has no port to check — add a port to the slice, or use a TCP or command check instead", idx, s.Name, unaimed))
	}

	return errs
}

// envKeyProblem describes why key is not a usable environment variable name,
// or returns "" when it is fine. The rule is delegated to apimachinery so it
// cannot drift from the Kubernetes semantics Mozza compiles against, but the
// wording is Mozza's own: apimachinery builds its message with RegexError,
// which quotes the raw pattern at the reader.
//
// IsEnvVarName is the deliberate choice over the wider IsRelaxedEnvVarName.
// The extra names the relaxed rule allows are only accepted by clusters running
// with the RelaxedEnvironmentVariableValidation feature gate enabled, and a
// recipe has to compile the same way everywhere.
func envKeyProblem(key string) string {
	if key == "" {
		return "must not be empty"
	}

	if len(validation.IsEnvVarName(key)) == 0 {
		return ""
	}

	// IsEnvVarName rejects for two unrelated reasons, so report only the one the
	// key actually broke rather than reciting both at every reader.
	if key == "." || strings.HasPrefix(key, "..") {
		return `must not be "." or ".." or start with ".."`
	}

	return `must contain only letters, digits, underscores, hyphens, and dots, ` +
		`and must not start with a digit (for example DATABASE_URL)`
}

// validateKindConstraints checks kind-specific semantic rules and new field
// constraints (multi-port, init steps, sidecars, schedule, mounts, etc.).
func validateKindConstraints(idx int, s Slice, allNames map[string]bool) []error {
	var errs []error

	// Scheduled kind must have Schedule.
	if s.Kind == SliceKindScheduled && s.Schedule == "" {
		errs = append(errs, fmt.Errorf("slice[%d] %q: scheduled slice requires a schedule", idx, s.Name))
	}

	// Task kind must have RunOnce.
	if s.Kind == SliceKindTask && !s.RunOnce {
		errs = append(errs, fmt.Errorf("slice[%d] %q: task slice requires run-once", idx, s.Name))
	}

	// Daemon kind must not have Replicas > 0.
	if s.Kind == SliceKindDaemon && s.Replicas > 0 {
		errs = append(errs, fmt.Errorf("slice[%d] %q: daemon slice runs on every node — replicas is not applicable", idx, s.Name))
	}

	// Daemon kind must not have AutoScale.
	if s.Kind == SliceKindDaemon && s.AutoScale != nil {
		errs = append(errs, fmt.Errorf("slice[%d] %q: daemon slice runs on every node — auto-scaling is not applicable", idx, s.Name))
	}

	// Stateful kind needs at least one stateful feature.
	if s.Kind == SliceKindStateful && s.StatefulStorage == "" && !s.OrderedStartup && !s.PeerDiscovery {
		errs = append(errs, fmt.Errorf("slice[%d] %q: stateful slice needs at least one stateful feature (storage, ordered startup, or peer discovery)", idx, s.Name))
	}

	// Multi-port names must be unique within a slice.
	if len(s.Ports) > 0 {
		portNames := make(map[string]bool, len(s.Ports))
		for _, p := range s.Ports {
			if p.Name != "" {
				if portNames[p.Name] {
					errs = append(errs, fmt.Errorf("slice[%d] %q: duplicate port name %q", idx, s.Name, p.Name))
				}
				portNames[p.Name] = true
			}
		}
	}

	// Init step images must not be empty.
	for j, is := range s.InitSteps {
		if is.Image == "" {
			errs = append(errs, fmt.Errorf("slice[%d] %q: init step[%d] image must not be empty", idx, s.Name, j))
		}
	}

	// Sidecar names must be unique within a slice.
	if len(s.Sidecars) > 0 {
		sidecarNames := make(map[string]bool, len(s.Sidecars))
		for _, sc := range s.Sidecars {
			if sc.Name != "" {
				if sidecarNames[sc.Name] {
					errs = append(errs, fmt.Errorf("slice[%d] %q: duplicate sidecar name %q", idx, s.Name, sc.Name))
				}
				sidecarNames[sc.Name] = true
			}
		}
	}

	// Schedule must look like a valid cron expression (5 space-separated fields).
	if s.Schedule != "" && !validCronFields.MatchString(strings.TrimSpace(s.Schedule)) {
		errs = append(errs, fmt.Errorf("slice[%d] %q: invalid schedule %q (must be 5-field cron expression)", idx, s.Name, s.Schedule))
	}

	// Mount target paths must start with "/".
	for _, m := range s.Mounts {
		if !strings.HasPrefix(m.Target, "/") {
			errs = append(errs, fmt.Errorf("slice[%d] %q: mount target %q must be an absolute path", idx, s.Name, m.Target))
		}
	}

	// NetworkPolicy AllowFrom must reference existing slice names.
	if s.NetworkPolicy != nil {
		for _, src := range s.NetworkPolicy.AllowFrom {
			if !allNames[src] {
				errs = append(errs, fmt.Errorf("slice[%d] %q: network policy references unknown source %q", idx, s.Name, src))
			}
		}
	}

	// AutoScale validation.
	if s.AutoScale != nil {
		if s.AutoScale.MinReplicas < 1 {
			errs = append(errs, fmt.Errorf("slice[%d] %q: auto-scale min replicas must be > 0", idx, s.Name))
		}
		if s.AutoScale.MaxReplicas < s.AutoScale.MinReplicas {
			errs = append(errs, fmt.Errorf("slice[%d] %q: auto-scale max must be >= min", idx, s.Name))
		}
		if s.AutoScale.CPUTarget != 0 && (s.AutoScale.CPUTarget < 1 || s.AutoScale.CPUTarget > 100) {
			errs = append(errs, fmt.Errorf("slice[%d] %q: auto-scale CPU target must be 1-100", idx, s.Name))
		}
		if s.AutoScale.MemoryTarget != 0 && (s.AutoScale.MemoryTarget < 1 || s.AutoScale.MemoryTarget > 100) {
			errs = append(errs, fmt.Errorf("slice[%d] %q: auto-scale memory target must be 1-100", idx, s.Name))
		}
	}

	// DisruptionBudget only on kinds with replicas.
	if s.DisruptionBudget != nil && !kindsWithReplicas[s.Kind] {
		errs = append(errs, fmt.Errorf("slice[%d] %q: disruption budget requires a workload with replicas", idx, s.Name))
	}

	// SecuritySpec RunAsUser must be >= 0 (only check when Security is set
	// and RunAsUser was explicitly provided; since 0 is a valid UID we treat
	// negative values as invalid).
	if s.Security != nil && s.Security.RunAsUser < 0 {
		errs = append(errs, fmt.Errorf("slice[%d] %q: run-as user must be >= 0", idx, s.Name))
	}

	// GracefulShutdown must be > 0 when set.
	if s.GracefulShutdown < 0 {
		errs = append(errs, fmt.Errorf("slice[%d] %q: graceful shutdown must be > 0 seconds", idx, s.Name))
	}

	return errs
}

// validateDependencies checks that all Needs references resolve to existing
// slice names and that no circular dependencies exist.
func validateDependencies(slices []Slice) []error {
	names := make(map[string]bool, len(slices))
	for _, s := range slices {
		if s.Name != "" {
			names[s.Name] = true
		}
	}

	var errs []error

	errs = append(errs, validateNeedsRefs(slices, names)...)
	errs = append(errs, validateNoCycles(slices, names)...)

	return errs
}

// validateNeedsRefs checks that every Needs entry points to an existing
// slice name.
func validateNeedsRefs(slices []Slice, names map[string]bool) []error {
	var errs []error

	for _, s := range slices {
		for _, need := range s.Needs {
			if !names[need] {
				errs = append(errs, fmt.Errorf("slice %q: needs unknown slice %q", s.Name, need))
			}
		}
	}

	return errs
}

// validateNoCycles detects circular dependencies among slices using
// depth-first search with a visited-state map.
func validateNoCycles(slices []Slice, names map[string]bool) []error {
	adj := sliceAdjacency(slices)

	// States: 0 = unvisited, 1 = in-progress, 2 = done.
	state := make(map[string]int, len(slices))
	var errs []error

	for _, s := range slices {
		if s.Name != "" && state[s.Name] == 0 {
			if err := dfsDetectCycle(s.Name, adj, state); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// sliceAdjacency builds a directed adjacency list from slice Needs fields.
func sliceAdjacency(slices []Slice) map[string][]string {
	adj := make(map[string][]string, len(slices))
	for _, s := range slices {
		adj[s.Name] = s.Needs
	}

	return adj
}

// dfsDetectCycle walks the dependency graph from the given node using
// depth-first search, returning an error if a cycle is detected.
func dfsDetectCycle(node string, adj map[string][]string, state map[string]int) error {
	state[node] = 1 // in-progress

	for _, dep := range adj[node] {
		switch state[dep] {
		case 1:
			return fmt.Errorf("circular dependency detected: %s -> %s", node, dep)
		case 0:
			if err := dfsDetectCycle(dep, adj, state); err != nil {
				return err
			}
		}
	}

	state[node] = 2 // done

	return nil
}
