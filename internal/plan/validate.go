package plan

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
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

	portErrs, primaryUnusable := validatePortRanges(idx, s)
	errs = append(errs, portErrs...)

	if s.Replicas < 0 {
		errs = append(errs, fmt.Errorf("slice[%d] %q: replicas must be >= 0, got %d", idx, s.Name, s.Replicas))
	}

	if s.HealthPath != "" && !validHealthPath.MatchString(s.HealthPath) {
		errs = append(errs, fmt.Errorf("slice[%d] %q: health path must start with / and contain only alphanumeric, hyphens, underscores, dots, and slashes", idx, s.Name))
	}

	errs = append(errs, validateProbeTargets(idx, s, primaryUnusable)...)

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

	errs = append(errs, validateQuantities(idx, s)...)

	// Kind-specific validation rules.
	errs = append(errs, validateKindConstraints(idx, s, allNames)...)

	return errs
}

// validatePortRanges reports every port on the slice that Kubernetes will not
// accept, so no recipe reaches a compiler that would emit a containerPort or a
// Service port the API server refuses.
//
// The second result reports whether one of the rejected ports was a carrier
// PrimaryPort reads (Slice.Port or an entry in Slice.Ports). That is what lets
// validateProbeTargets tell a slice that declares no port from one whose declared
// port was thrown out, and say only the latter. A rejected sidecar or probe port
// is deliberately not counted: neither is what a health check would have aimed at.
//
// Zero is legal on Slice.Port and ProbeSpec.TCPPort and an error everywhere else,
// because those two fields use zero to mean "not set": plan.go documents
// Slice.Port's zero as "no port" and TCPPort's as "inherit the slice's primary
// port". Three tests depend on it: a worker slice built with Port: 0 must still
// compile to a Deployment with no Service (internal/k8s/compiler_test.go:213,
// internal/local/compiler_test.go:246, internal/doctor/rule_test.go:220).
//
// A PortSpec entry carries no such meaning: it exists only because a port was
// declared, so zero there is always a mistake.
//
// That leaves the one case this function cannot judge: an author writing the
// literal 0 for a slice or probe port. By the time a plan exists that is
// indistinguishable from writing nothing, so the parser rejects it while it can
// still tell the difference and name the line.
func validatePortRanges(idx int, s Slice) ([]error, bool) {
	var errs []error

	primaryUnusable := false

	if s.Port != 0 && !usablePort(s.Port) {
		errs = append(errs, fmt.Errorf("slice[%d] %q: port is %d — a port must be between 1 and 65535", idx, s.Name, s.Port))
		primaryUnusable = true
	}

	for j, ps := range s.Ports {
		if !usablePort(ps.Port) {
			errs = append(errs, fmt.Errorf("slice[%d] %q: %s is %d — a port must be between 1 and 65535", idx, s.Name, namedSubject("port", ps.Name, j), ps.Port))
			primaryUnusable = true
		}
	}

	for j, sc := range s.Sidecars {
		for k, ps := range sc.Ports {
			if !usablePort(ps.Port) {
				errs = append(errs, fmt.Errorf("slice[%d] %q: %s %s is %d — a port must be between 1 and 65535", idx, s.Name, namedSubject("sidecar", sc.Name, j), namedSubject("port", ps.Name, k), ps.Port))
			}
		}
	}

	for _, ps := range s.Probes {
		if ps.TCPPort != 0 && !usablePort(ps.TCPPort) {
			errs = append(errs, fmt.Errorf("slice[%d] %q: %s's port is %d — a port must be between 1 and 65535", idx, s.Name, probeSubject(ps), ps.TCPPort))
		}
	}

	return errs, primaryUnusable
}

// usablePort reports whether n is a port number Kubernetes will accept.
func usablePort(n int) bool {
	return n >= 1 && n <= 65535
}

// namedSubject describes one entry of a repeated field by its name when it has
// one and by its position when it does not, so a message can point at the right
// entry either way: `port "http"` or `port[1]`, following the `init step[%d]`
// idiom used elsewhere in this file.
func namedSubject(noun, name string, idx int) string {
	if name == "" {
		return fmt.Sprintf("%s[%d]", noun, idx)
	}

	return fmt.Sprintf("%s %q", noun, name)
}

// probeSubject names the probe a message is about. The parser always sets Type,
// but a hand-built ProbeSpec reaching the exported compilers need not, so there
// is a fallback that still reads as English.
func probeSubject(ps ProbeSpec) string {
	if ps.Type == "" {
		return "this probe"
	}

	return ps.Type + " check"
}

// validateProbeTargets checks that every health check on a slice can actually be
// aimed: an HTTP check needs a port to connect to, and any check needs something
// to check. Compilers would otherwise have to invent a target or drop the check,
// and both hide the mistake until the rollout hangs.
//
// One "health check <path>" directive lands in three places (HealthPath, plus a
// readiness and a liveness ProbeSpec carrying the same path), so the no-port
// complaint is collected and reported once per slice rather than once per copy.
//
// primaryUnusable says the slice did declare a port and validatePortRanges
// rejected it. There is then nothing left to tell the author: the port they wrote
// is already reported, and adding "the health check has no port" on top of it
// describes a consequence of that one mistake as if it were a second one.
func validateProbeTargets(idx int, s Slice, primaryUnusable bool) []error {
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
			errs = append(errs, fmt.Errorf("slice[%d] %q: %s must say what to check — give it an HTTP path, a TCP port, or a command", idx, s.Name, probeSubject(ps)))
		}
	}

	if unaimed != "" && !primaryUnusable {
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

// validateQuantities checks every field that a compiler turns into a Kubernetes
// resource.Quantity. Each of those fields reaches resource.MustParse, which
// panics rather than returning an error, so a value that is not a quantity has
// to be stopped here or it takes the whole command down with a stack trace
// instead of a sentence.
//
// An empty value is skipped rather than rejected: the storage fields use "" to
// mean "no volume" (pvc.go and statefulset.go both read it that way), so every
// storage-free recipe depends on empty staying valid.
func validateQuantities(idx int, s Slice) []error {
	var errs []error

	// Subject nouns, not field names: the message is read by someone who wrote a
	// recipe and has never seen the struct.
	type quantityField struct {
		subject string
		value   string
	}

	fields := []quantityField{{"per-copy storage", s.StatefulStorage}}

	if s.Resources != nil {
		fields = append(fields,
			quantityField{"cpu limit", s.Resources.CPULimit},
			quantityField{"memory limit", s.Resources.MemoryLimit},
		)
	}

	if s.Database != nil {
		fields = append(fields, quantityField{"database storage", s.Database.Storage})
	}

	if s.Cache != nil {
		fields = append(fields, quantityField{"cache storage", s.Cache.Storage})
	}

	for _, f := range fields {
		if f.value == "" {
			continue
		}

		if problem := quantityProblem(f.value); problem != "" {
			errs = append(errs, fmt.Errorf("slice[%d] %q: %s %q %s", idx, s.Name, f.subject, f.value, problem))
		}
	}

	return errs
}

// quantityProblem describes why value is not a usable Kubernetes quantity, or
// returns "" when it is fine. Like envKeyProblem, the rule is delegated to
// apimachinery so it cannot drift from the semantics Mozza compiles against,
// while the wording stays Mozza's own: ParseQuantity's ErrFormatWrong quotes the
// raw regular expression at the reader, which is the string that appears in the
// panic this rule exists to prevent. The error is consulted, never wrapped.
//
// A leftover template placeholder is reported before the syntax rule. It is not
// a quantity either, but telling an author that "{{STORAGE_SIZE}}" fails a
// number format sends them to fix the wrong thing: the value was never meant to
// survive rendering.
func quantityProblem(value string) string {
	if unrenderedPlaceholder(value) {
		return "is still a template variable — substitute a real value before deploying"
	}

	q, err := resource.ParseQuantity(value)
	if err != nil {
		return "is not a quantity Kubernetes understands — write a number with an optional unit, " +
			"for example 10Gi, 512Mi or 500m"
	}

	// Kubernetes' own rule is non-negative rather than positive, so zero is
	// deliberately allowed; a stricter rule here would be Mozza inventing one.
	if q.Sign() < 0 {
		return "is negative — a quantity must be zero or more"
	}

	return ""
}

// unrenderedPlaceholder reports whether value still carries a template variable
// the renderer was meant to substitute. RenderRecipe replaces the literal
// "{{"+key+"}}", so that pair is exactly the shape a leftover takes, and no
// Kubernetes quantity can contain a brace, so there is no false positive to weigh.
func unrenderedPlaceholder(value string) bool {
	_, rest, ok := strings.Cut(value, "{{")

	return ok && strings.Contains(rest, "}}")
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
	errs = append(errs, validateNoCycles(slices)...)

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
func validateNoCycles(slices []Slice) []error {
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
