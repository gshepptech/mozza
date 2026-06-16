package plan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gshepptech/mozza/internal/recipe"
)

// validKinds maps raw kind strings to their typed SliceKind constants.
var validKinds = map[string]SliceKind{
	"web":       SliceKindWeb,
	"api":       SliceKindAPI,
	"worker":    SliceKindWorker,
	"task":      SliceKindTask,
	"scheduled": SliceKindScheduled,
	"database":  SliceKindDatabase,
	"cache":     SliceKindCache,
	"stateful":  SliceKindStateful,
	"gateway":   SliceKindGateway,
	"daemon":    SliceKindDaemon,
}

// databaseEngines maps engine names to their kind for inference.
var databaseEngines = map[string]bool{ //nolint:gochecknoglobals // immutable lookup table
	"postgres": true,
	"mysql":    true,
	"mongo":    true,
}

// cacheEngines maps engine names to their kind for inference.
var cacheEngines = map[string]bool{ //nolint:gochecknoglobals // immutable lookup table
	"redis":     true,
	"memcached": true,
}

// engineDefaults maps engine names to their default image, port, and mount path.
var engineDefaults = map[string]engineDefault{ //nolint:gochecknoglobals // immutable lookup table
	"postgres":  {imageSuffix: "-alpine", port: 5432, mountPath: "/var/lib/postgresql/data"},
	"mysql":     {imageSuffix: "", port: 3306, mountPath: "/var/lib/mysql"},
	"mongo":     {imageSuffix: "", port: 27017, mountPath: "/data/db"},
	"redis":     {imageSuffix: "-alpine", port: 6379, mountPath: "/data"},
	"memcached": {imageSuffix: "-alpine", port: 11211, mountPath: "/data"},
}

// engineDefault holds default image suffix, port, and mount path for an engine.
type engineDefault struct {
	imageSuffix string
	port        int
	mountPath   string
}

// workerNameHints are substrings that indicate a worker slice.
var workerNameHints = []string{"worker", "job", "processor", "cron"} //nolint:gochecknoglobals // immutable lookup table

// gatewayNameHints are substrings that indicate a gateway/proxy slice.
var gatewayNameHints = []string{"gateway", "proxy"} //nolint:gochecknoglobals // immutable lookup table

// apiNameHints are substrings that indicate an API slice.
var apiNameHints = []string{"api"} //nolint:gochecknoglobals // immutable lookup table

// autoWireEngineEnvVars maps engine names to the env var name and URL pattern
// used for auto-wiring dependency injection.
var autoWireEngineEnvVars = map[string]struct {
	envVar  string
	pattern string // %s = slice name, %s = app name (if needed)
}{
	"postgres":  {envVar: "DATABASE_URL", pattern: "postgres://%s:5432/%s"},
	"mysql":     {envVar: "DATABASE_URL", pattern: "mysql://%s:3306/%s"},
	"mongo":     {envVar: "MONGO_URL", pattern: "mongodb://%s:27017/%s"},
	"redis":     {envVar: "REDIS_URL", pattern: "redis://%s:6379"},
	"memcached": {envVar: "MEMCACHED_URL", pattern: "%s:11211"},
}

// Build transforms a Recipe AST into an AppPlan intermediate representation.
// It maps recipe slices to plan slices, infers kinds and images from engine
// shorthands, populates Database/Cache specs, and builds the Ingredients
// dependency graph.
func Build(r *recipe.Recipe) (*AppPlan, error) {
	if r == nil {
		return nil, fmt.Errorf("Build: %w", errors.New("recipe must not be nil"))
	}

	// Resolve image aliases before building the plan. If a slice's Image
	// matches a key in Recipe.Aliases, replace it with the full image ref.
	resolveAliases(r)

	sliceIndex := buildSliceIndex(r.Slices)

	slices, sliceErrs := convertSlices(r.Slices)
	needsErrs := validateRecipeNeeds(r.Slices, sliceIndex)
	ingredients := buildIngredients(r.Slices)

	if err := errors.Join(append(sliceErrs, needsErrs...)...); err != nil {
		return nil, fmt.Errorf("Build: %w", err)
	}

	// Auto-wire dependency injection env vars after all slices are converted.
	autoWireDependencies(slices, r.Slices, r.Name)

	return &AppPlan{
		Name:        r.Name,
		Namespace:   r.Namespace,
		Slices:      slices,
		Ingredients: ingredients,
		CRDs:        r.CRDs,
	}, nil
}

// buildSliceIndex creates a set of known slice names for fast lookup.
func buildSliceIndex(slices []recipe.Slice) map[string]bool {
	index := make(map[string]bool, len(slices))
	for _, s := range slices {
		index[s.Name] = true
	}
	return index
}

// convertSlices transforms recipe slices into plan slices, collecting
// any conversion errors encountered along the way.
func convertSlices(slices []recipe.Slice) ([]Slice, []error) {
	result := make([]Slice, 0, len(slices))
	var errs []error

	for _, s := range slices {
		ps, err := convertSlice(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result = append(result, ps)
	}

	return result, errs
}

// convertSlice transforms a single recipe.Slice into a plan.Slice.
func convertSlice(s recipe.Slice) (Slice, error) {
	ps := Slice{
		Name:             strings.ToLower(s.Name),
		Image:            s.Image,
		Port:             s.Port,
		Public:           s.Public,
		Replicas:         s.Replicas,
		HealthPath:       s.Health,
		Needs:            s.Needs,
		Env:              s.Env,
		RestartPolicy:    s.RestartPolicy,
		Domain:           s.Domain,
		PullSecret:       s.PullSecret,
		ServiceAccount:   s.ServiceAccount,
		GracefulShutdown: s.GracefulShutdown,
		Schedule:         s.Schedule,
		RunOnce:          s.RunOnce,
		Parallelism:      s.Parallelism,
		Retries:          s.Retries,
		DaemonMode:       s.DaemonMode,
		OrderedStartup:   s.OrderedStartup,
		PeerDiscovery:    s.PeerDiscovery,
		StatefulStorage:  s.StatefulStorage,
		DNSName:          s.DNSName,
	}

	// Convert recipe secret refs to plan secret refs.
	for _, sr := range s.Secrets {
		ps.Secrets = append(ps.Secrets, SecretRef{
			EnvVar:     sr.EnvVar,
			SecretName: sr.SecretName,
			Key:        sr.Key,
		})
	}

	// Convert multi-port specs.
	for _, p := range s.Ports {
		ps.Ports = append(ps.Ports, PortSpec{
			Name:     p.Name,
			Port:     p.Port,
			Protocol: p.Protocol,
		})
	}

	// Backward compatibility: if Ports is set, use first port for legacy Port field.
	if len(ps.Ports) > 0 && ps.Port == 0 {
		ps.Port = ps.Ports[0].Port
	}

	// Convert probe specs.
	for _, p := range s.Probes {
		ps.Probes = append(ps.Probes, ProbeSpec{
			Type:     p.Type,
			HTTPPath: p.HTTPPath,
			Command:  p.Command,
			TCPPort:  p.TCPPort,
			Interval: p.Interval,
			Timeout:  p.Timeout,
			Delay:    p.Delay,
		})
	}

	// Backward compatibility: if Probes is set, populate legacy HealthPath from
	// the first readiness probe's HTTPPath.
	if ps.HealthPath == "" && len(ps.Probes) > 0 {
		for _, probe := range ps.Probes {
			if probe.Type == "readiness" && probe.HTTPPath != "" {
				ps.HealthPath = probe.HTTPPath
				break
			}
		}
	}

	// Convert init steps.
	for _, is := range s.InitSteps {
		ps.InitSteps = append(ps.InitSteps, InitStep{
			Image:   is.Image,
			Command: is.Command,
			Env:     is.Env,
		})
	}

	// Convert sidecars.
	for _, sc := range s.Sidecars {
		sidecar := Sidecar{
			Name:  sc.Name,
			Image: sc.Image,
			Env:   sc.Env,
		}
		for _, p := range sc.Ports {
			sidecar.Ports = append(sidecar.Ports, PortSpec{
				Name:     p.Name,
				Port:     p.Port,
				Protocol: p.Protocol,
			})
		}
		ps.Sidecars = append(ps.Sidecars, sidecar)
	}

	// Convert mounts.
	for _, m := range s.Mounts {
		ps.Mounts = append(ps.Mounts, MountSpec{
			Type:     m.Type,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}

	// Convert lifecycle.
	if s.Lifecycle != nil {
		ps.Lifecycle = &LifecycleSpec{
			PreStopCommand:   s.Lifecycle.PreStopCommand,
			PreStopWait:      s.Lifecycle.PreStopWait,
			PostStartCommand: s.Lifecycle.PostStartCommand,
		}
	}

	// Convert permissions.
	for _, perm := range s.Permissions {
		ps.Permissions = append(ps.Permissions, Permission{
			Verbs:       perm.Verbs,
			Resources:   perm.Resources,
			Namespace:   perm.Namespace,
			ClusterWide: perm.ClusterWide,
		})
	}

	// Convert scheduling.
	if s.Scheduling != nil {
		sched := &SchedulingSpec{
			SpreadTopology: s.Scheduling.SpreadTopology,
			AntiAffinity:   s.Scheduling.AntiAffinity,
		}
		for _, np := range s.Scheduling.NodePreferences {
			sched.NodePreferences = append(sched.NodePreferences, LabelConstraint{
				Key:   np.Key,
				Value: np.Value,
			})
		}
		for _, nr := range s.Scheduling.NodeRequirements {
			sched.NodeRequirements = append(sched.NodeRequirements, LabelConstraint{
				Key:   nr.Key,
				Value: nr.Value,
			})
		}
		ps.Scheduling = sched
	}

	// Convert network policy.
	if s.NetworkPolicy != nil {
		ps.NetworkPolicy = &NetworkPolicySpec{
			AllowFrom:      s.NetworkPolicy.AllowFrom,
			AllowNamespace: s.NetworkPolicy.AllowNamespace,
			DenyAll:        s.NetworkPolicy.DenyAll,
		}
	}

	// Convert auto-scale.
	if s.AutoScale != nil {
		ps.AutoScale = &AutoScaleSpec{
			MinReplicas:  s.AutoScale.MinReplicas,
			MaxReplicas:  s.AutoScale.MaxReplicas,
			CPUTarget:    s.AutoScale.CPUTarget,
			MemoryTarget: s.AutoScale.MemoryTarget,
			CustomMetric: s.AutoScale.CustomMetric,
			CustomTarget: s.AutoScale.CustomTarget,
		}
	}

	// Convert disruption budget.
	if s.DisruptionBudget != nil {
		ps.DisruptionBudget = &DisruptionBudgetSpec{
			MinAvailable:   s.DisruptionBudget.MinAvailable,
			MaxUnavailable: s.DisruptionBudget.MaxUnavailable,
		}
	}

	// Convert security spec.
	if s.Security != nil {
		ps.Security = &SecuritySpec{
			RunAsUser:        s.Security.RunAsUser,
			RunAsGroup:       s.Security.RunAsGroup,
			ReadOnlyRoot:     s.Security.ReadOnlyRoot,
			DropCapabilities: s.Security.DropCapabilities,
			AddCapabilities:  s.Security.AddCapabilities,
		}
	}

	// Convert update strategy.
	if s.UpdateStrategy != nil {
		ps.UpdateStrategy = &UpdateStrategySpec{
			MaxSurge:       s.UpdateStrategy.MaxSurge,
			MaxUnavailable: s.UpdateStrategy.MaxUnavailable,
		}
	}

	if s.CPULimit != "" || s.MemoryLimit != "" {
		ps.Resources = &ResourceSpec{
			CPULimit:    s.CPULimit,
			MemoryLimit: s.MemoryLimit,
		}
	}

	applyEngineDefaults(&ps, s)
	inferKind(&ps, s)

	if ps.Kind == "" {
		return Slice{}, fmt.Errorf("slice %q: could not determine kind", ps.Name)
	}

	populateStorageSpec(&ps, s)

	return ps, nil
}

// applyEngineDefaults sets image, port from engine shorthand when present.
func applyEngineDefaults(ps *Slice, s recipe.Slice) {
	if s.Engine == "" {
		return
	}

	defaults, ok := engineDefaults[s.Engine]
	if !ok {
		return
	}

	if ps.Image == "" {
		version := s.Version
		if version == "" {
			version = "latest"
		}
		ps.Image = s.Engine + ":" + version + defaults.imageSuffix
	}

	if ps.Port == 0 {
		ps.Port = defaults.port
	}
}

// inferKind determines the slice kind using a 12-rule priority list.
// First match wins:
//
//  1. Explicit kind from recipe → use it
//  2. Engine = postgres/mysql/mongo → database
//  3. Engine = redis/memcached → cache
//  4. Has Schedule → scheduled
//  5. Has RunOnce → task
//  6. Has DaemonMode → daemon
//  7. Has StatefulStorage or OrderedStartup or PeerDiscovery → stateful
//  8. Has Public + port → web
//  9. Has port + name contains api/gateway/proxy → api (or gateway)
//  10. Has port → api
//  11. Name contains worker/job/processor/cron → worker
//  12. Image + no port → worker
func inferKind(ps *Slice, s recipe.Slice) {
	// Rule 1: Explicit kind.
	if s.Kind != "" {
		if kind, ok := validKinds[s.Kind]; ok {
			ps.Kind = kind
		}
		return
	}

	// Rule 2: Database engine shorthand.
	if databaseEngines[s.Engine] {
		ps.Kind = SliceKindDatabase
		return
	}

	// Rule 3: Cache engine shorthand.
	if cacheEngines[s.Engine] {
		ps.Kind = SliceKindCache
		return
	}

	// Rule 4: Has Schedule → scheduled.
	if s.Schedule != "" {
		ps.Kind = SliceKindScheduled
		return
	}

	// Rule 5: Has RunOnce → task.
	if s.RunOnce {
		ps.Kind = SliceKindTask
		return
	}

	// Rule 6: Has DaemonMode → daemon.
	if s.DaemonMode {
		ps.Kind = SliceKindDaemon
		return
	}

	// Rule 7: Has stateful features → stateful.
	if s.StatefulStorage != "" || s.OrderedStartup || s.PeerDiscovery {
		ps.Kind = SliceKindStateful
		return
	}

	// Rule 8: Public + port → web.
	if ps.Public && ps.Port > 0 {
		ps.Kind = SliceKindWeb
		return
	}

	// Rule 9: Has port + name contains gateway/proxy → gateway; api → api.
	lower := strings.ToLower(s.Name)
	if ps.Port > 0 {
		for _, hint := range gatewayNameHints {
			if strings.Contains(lower, hint) {
				ps.Kind = SliceKindGateway
				return
			}
		}
		for _, hint := range apiNameHints {
			if strings.Contains(lower, hint) {
				ps.Kind = SliceKindAPI
				return
			}
		}
	}

	// Rule 10: Has port → api.
	if ps.Port > 0 {
		ps.Kind = SliceKindAPI
		return
	}

	// Rule 11: Name contains worker/job/processor/cron hints.
	for _, hint := range workerNameHints {
		if strings.Contains(lower, hint) {
			ps.Kind = SliceKindWorker
			return
		}
	}

	// Rule 12: Image with no port → worker.
	if ps.Image != "" && ps.Port == 0 {
		ps.Kind = SliceKindWorker
		return
	}
}

// populateStorageSpec sets the Database or Cache spec on a plan slice
// when the recipe slice declares a storage size or engine shorthand.
func populateStorageSpec(ps *Slice, s recipe.Slice) {
	mountPath := ""
	if defaults, ok := engineDefaults[s.Engine]; ok {
		mountPath = defaults.mountPath
	}

	switch ps.Kind {
	case SliceKindDatabase:
		ps.Database = &DatabaseSpec{
			Storage:      s.Storage,
			BackupPolicy: s.Backups,
			MountPath:    mountPath,
		}
	case SliceKindCache:
		if s.Storage != "" {
			ps.Cache = &CacheSpec{
				Storage:   s.Storage,
				MountPath: mountPath,
			}
		}
	case SliceKindWeb, SliceKindWorker, SliceKindAPI, SliceKindTask,
		SliceKindScheduled, SliceKindStateful, SliceKindGateway,
		SliceKindDaemon:
		// No storage spec for these kinds.
	}
}

// validateRecipeNeeds checks that every needs reference in the recipe points
// to an existing slice name.
func validateRecipeNeeds(slices []recipe.Slice, index map[string]bool) []error {
	var errs []error

	for _, s := range slices {
		for _, need := range s.Needs {
			if !index[need] {
				errs = append(errs, fmt.Errorf(
					"slice %q: needs unknown slice %q", s.Name, need,
				))
			}
		}
	}

	return errs
}

// resolveAliases replaces image alias references in slices with their
// full image references from the recipe's Aliases map.
func resolveAliases(r *recipe.Recipe) {
	if len(r.Aliases) == 0 {
		return
	}

	for i := range r.Slices {
		if ref, ok := r.Aliases[r.Slices[i].Image]; ok {
			r.Slices[i].Image = ref
		}
	}
}

// buildIngredients constructs the dependency graph from all needs
// references across the recipe slices.
func buildIngredients(slices []recipe.Slice) []Ingredient {
	var ingredients []Ingredient

	for _, s := range slices {
		for _, need := range s.Needs {
			ingredients = append(ingredients, Ingredient{
				From: s.Name,
				To:   need,
			})
		}
	}

	return ingredients
}

// autoWireDependencies injects connection environment variables into slices
// that depend on database/cache/service slices. If the dependent slice already
// has the env var set, it is not overwritten.
func autoWireDependencies(planSlices []Slice, recipeSlices []recipe.Slice, appName string) {
	// Build index of plan slices by name for dependency lookup.
	sliceByName := make(map[string]*Slice, len(planSlices))
	for i := range planSlices {
		sliceByName[planSlices[i].Name] = &planSlices[i]
	}

	// Build index of recipe slices by name for engine lookup.
	recipeByName := make(map[string]*recipe.Slice, len(recipeSlices))
	for i := range recipeSlices {
		recipeByName[strings.ToLower(recipeSlices[i].Name)] = &recipeSlices[i]
	}

	for i := range planSlices {
		ps := &planSlices[i]
		for _, need := range ps.Needs {
			depSlice, ok := sliceByName[need]
			if !ok {
				continue
			}
			recDep := recipeByName[need]

			envVar, envVal := autoWireEnvVar(need, depSlice, recDep, appName)
			if envVar == "" {
				continue
			}

			// Don't overwrite existing env vars.
			if ps.Env == nil {
				ps.Env = make(map[string]string)
			}
			if _, exists := ps.Env[envVar]; !exists {
				ps.Env[envVar] = envVal
			}
		}
	}
}

// autoWireEnvVar returns the env var name and value to inject for a dependency,
// or empty strings if no auto-wiring applies.
func autoWireEnvVar(depName string, depSlice *Slice, recDep *recipe.Slice, appName string) (string, string) {
	// Check if the dependency has a known engine for specific auto-wire rules.
	if recDep != nil && recDep.Engine != "" {
		if aw, ok := autoWireEngineEnvVars[recDep.Engine]; ok {
			var val string
			switch recDep.Engine {
			case "redis":
				val = fmt.Sprintf(aw.pattern, depName)
			case "memcached":
				val = fmt.Sprintf(aw.pattern, depName)
			default:
				val = fmt.Sprintf(aw.pattern, depName, appName)
			}
			return aw.envVar, val
		}
	}

	// Generic service with port: inject <SLICE_NAME>_URL=http://slice:port.
	if depSlice.Port > 0 {
		envVar := strings.ToUpper(strings.ReplaceAll(depName, "-", "_")) + "_URL"
		envVal := fmt.Sprintf("http://%s:%d", depName, depSlice.Port)
		return envVar, envVal
	}

	return "", ""
}
