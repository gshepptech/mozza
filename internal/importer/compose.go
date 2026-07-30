package importer

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/recipe"
)

// composeFile is a representation of a docker-compose.yml (v3.x).
type composeFile struct {
	Services map[string]composeService        `yaml:"services"`
	Volumes  map[string]composeVolumeConfig   `yaml:"volumes"`
	Networks map[string]composeNetworkConfig  `yaml:"networks"`
	Configs  map[string]composeExternalConfig `yaml:"configs"`
	Secrets  map[string]composeExternalConfig `yaml:"secrets"`
}

// composeService represents a single service in a docker-compose file.
type composeService struct {
	Image       string              `yaml:"image"`
	Build       interface{}         `yaml:"build"` // string or map
	Ports       []string            `yaml:"ports"`
	DependsOn   interface{}         `yaml:"depends_on"` // []string or map[string]...
	Environment interface{}         `yaml:"environment"`
	Volumes     []string            `yaml:"volumes"`
	Restart     string              `yaml:"restart"`
	Deploy      *composeDeploy      `yaml:"deploy"`
	Healthcheck *composeHealthcheck `yaml:"healthcheck"`
	Networks    interface{}         `yaml:"networks"` // []string or map[string]...
	Configs     []composeConfigRef  `yaml:"configs"`
	Secrets     []composeSecretRef  `yaml:"secrets"`
}

type composeDeploy struct {
	Replicas int `yaml:"replicas"`
}

type composeHealthcheck struct {
	Test []string `yaml:"test"`
}

type composeVolumeConfig struct {
	Driver string `yaml:"driver"`
}

type composeNetworkConfig struct {
	Driver   string `yaml:"driver"`
	Internal bool   `yaml:"internal"`
}

type composeExternalConfig struct {
	File     string `yaml:"file"`
	External bool   `yaml:"external"`
}

// composeConfigRef is a config reference within a service.
type composeConfigRef struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// composeSecretRef is a secret reference within a service.
type composeSecretRef struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// Warning represents a non-fatal issue found during compose import.
type Warning struct {
	Feature  string `json:"feature"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "info", "warn", "error"
}

// knownEngines maps well-known Docker image prefixes to recipe engine shorthands.
var knownEngines = map[string]string{
	"postgres": "postgres",
	"redis":    "redis",
	"mysql":    "mysql",
	"mariadb":  "mysql",
}

// ComposeToRecipeAST converts docker-compose.yml content into a Recipe AST
// along with any warnings about unsupported features.
func ComposeToRecipeAST(yamlContent []byte) (*recipe.Recipe, []Warning, error) {
	var cf composeFile
	if err := yaml.Unmarshal(yamlContent, &cf); err != nil {
		return nil, nil, fmt.Errorf("parsing compose YAML: %w", err)
	}
	if len(cf.Services) == 0 {
		return nil, nil, fmt.Errorf("no services found in compose file")
	}

	names := sortedKeys(cf.Services)

	appName := names[0]
	if len(names) > 1 {
		appName = names[0] + "-stack"
	}

	r := &recipe.Recipe{
		Name:   appName,
		Slices: make([]recipe.Slice, 0, len(names)),
	}
	var warnings []Warning

	// Collect named volumes defined at top level.
	namedVolumes := make(map[string]bool, len(cf.Volumes))
	for v := range cf.Volumes {
		namedVolumes[v] = true
	}

	for _, name := range names {
		svc := cf.Services[name]
		slice, svcWarnings := convertService(name, svc, namedVolumes)
		r.Slices = append(r.Slices, slice)
		warnings = append(warnings, svcWarnings...)
	}

	// Warn about networks.
	warnings = append(warnings, convertNetworks(cf.Networks)...)

	// Warn about top-level configs.
	for cname := range cf.Configs {
		warnings = append(warnings, Warning{
			Feature:  "configs",
			Message:  fmt.Sprintf("config %q mapped as environment reference — verify mount path", cname),
			Severity: "info",
		})
	}

	return r, warnings, nil
}

// ComposeToRecipe converts docker-compose.yml content into a .mozza recipe string.
// This is the legacy API used by the scanner; it delegates to ComposeToRecipeAST
// internally and returns the text representation.
func ComposeToRecipe(composeYAML string) (string, error) {
	var cf composeFile
	if err := yaml.Unmarshal([]byte(composeYAML), &cf); err != nil {
		return "", fmt.Errorf("parsing compose YAML: %w", err)
	}
	if len(cf.Services) == 0 {
		return "", fmt.Errorf("no services found in compose file")
	}

	names := sortedKeys(cf.Services)

	appName := names[0]
	if len(names) > 1 {
		appName = names[0] + "-stack"
	}

	// Collect named volumes.
	namedVolumes := make(map[string]bool, len(cf.Volumes))
	for v := range cf.Volumes {
		namedVolumes[v] = true
	}

	var b strings.Builder
	fmt.Fprintf(&b, "App: %s\n", appName)

	for _, name := range names {
		svc := cf.Services[name]
		b.WriteString("\n")
		b.WriteString(capitalize(name) + ":\n")

		image := svc.Image
		if image == "" {
			if svc.Build != nil {
				b.WriteString("  # WARNING: build context detected, needs pre-built image\n")
			} else {
				b.WriteString("  # WARNING: no image specified, build required\n")
			}
			continue
		}

		// Check if this is a known engine (postgres, redis, mysql).
		engineName, version := parseEngineImage(image)
		if engineName != "" {
			line := "  " + engineName + " " + version
			if size := storageFor(engineName, svc.Volumes, namedVolumes); size != "" {
				line += ", " + size
			}
			b.WriteString(line + "\n")
		} else {
			fmt.Fprintf(&b, "  from image %s\n", image)
		}

		// Ports.
		publishedPort := 0
		for _, p := range svc.Ports {
			writePort(&b, p)
			if n, _ := parsePort(p); publishedPort == 0 && n > 0 {
				publishedPort = n
			}
		}

		// Health check.
		if svc.Healthcheck != nil {
			writeHealthcheck(&b, svc.Healthcheck, publishedPort)
		}

		// Replicas.
		if svc.Deploy != nil && svc.Deploy.Replicas > 1 {
			fmt.Fprintf(&b, "  run %d copies\n", svc.Deploy.Replicas)
		}

		// Restart policy.
		if svc.Restart != "" && svc.Restart != "no" {
			fmt.Fprintf(&b, "  restart %s\n", svc.Restart)
		}

		// depends_on -> needs.
		deps := parseDependsOn(svc.DependsOn)
		if len(deps) > 0 {
			fmt.Fprintf(&b, "  needs %s\n", strings.Join(deps, " and "))
		}

		// Named volumes.
		writeVolumes(&b, engineName, svc.Volumes, namedVolumes)

		// Environment variables.
		envs := parseEnvironment(svc.Environment)
		envKeys := sortedKeys(envs)
		for _, k := range envKeys {
			fmt.Fprintf(&b, "  set %s to \"%s\"\n", k, envs[k])
		}

		// Secrets.
		for _, s := range svc.Secrets {
			fmt.Fprintf(&b, "  secret %s from %s\n",
				strings.ToUpper(strings.ReplaceAll(s.Source, "-", "_")),
				s.Source)
		}
	}

	return b.String(), nil
}

// convertService converts a single compose service to a recipe Slice.
func convertService(
	name string,
	svc composeService,
	namedVolumes map[string]bool,
) (recipe.Slice, []Warning) {
	var warnings []Warning

	sliceName := capitalize(name)
	slice := recipe.Slice{Name: sliceName}

	// Image / build.
	if svc.Image == "" {
		if svc.Build != nil {
			warnings = append(warnings, Warning{
				Feature:  "build",
				Message:  fmt.Sprintf("service %q uses build context — needs a pre-built image", name),
				Severity: "warn",
			})
		}
		return slice, warnings
	}

	engineName, version := parseEngineImage(svc.Image)
	if engineName != "" {
		slice.Engine = engineName
		slice.Version = version
		slice.Storage = storageFor(engineName, svc.Volumes, namedVolumes)
	} else {
		slice.Image = svc.Image
	}

	// Ports.
	for _, p := range svc.Ports {
		port, public := parsePort(p)
		if port > 0 {
			if slice.Port == 0 {
				slice.Port = port
			}
			if public {
				slice.Public = true
			}
		}
	}

	// Health check. The healthcheck URL carries the port the service really
	// listens on, so it can supply one when no published port did. Without a port
	// the check cannot be aimed, and importing it anyway would write a recipe that
	// mozza validate rejects — so the check is dropped and the user told why.
	if svc.Healthcheck != nil {
		path, urlPort := extractHealthCheck(svc.Healthcheck)
		if slice.Port == 0 {
			slice.Port = urlPort
		}
		switch {
		case path == "":
			// Not an HTTP health check — there is nothing to carry into the recipe.
		case slice.Port == 0:
			warnings = append(warnings, Warning{
				Feature: "healthcheck",
				Message: fmt.Sprintf(
					"service %q health check %s dropped — no published port and none in the check URL; publish the port in compose and import again",
					name, path),
				Severity: "warn",
			})
		default:
			slice.Health = path
		}
	}

	// Replicas.
	if svc.Deploy != nil && svc.Deploy.Replicas > 1 {
		slice.Replicas = svc.Deploy.Replicas
	}

	// Restart policy.
	if svc.Restart != "" && svc.Restart != "no" {
		slice.RestartPolicy = svc.Restart
	}

	// depends_on.
	slice.Needs = parseDependsOn(svc.DependsOn)

	// Named volumes. The size a volume implies is already on the engine shorthand
	// (storageFor, above), the only storage form the importer emits — so an
	// imported service gets one volume at most, at its engine's own path, and
	// everything compose declared beyond that is said aloud rather than left to be
	// found as missing persistence in a running cluster. The engine-less warning
	// below names the per-copy storage directive a reader can add by hand.
	//
	// No Mounts entry goes with any of it. A recipe's mounts name an object that
	// already exists — a pvc mount compiles to a claimName taken verbatim from the
	// source (k8s/deployment.go addMounts) — and mozza creates no claim under a
	// compose volume's name, so the pod would wait on one that is never made. That
	// is why the text builder writes a comment here and why this path must leave
	// the same nothing behind it: the two are one import, and a mount on one of
	// them is a volume the other never asked for.
	for _, v := range svc.Volumes {
		volName, mountPath := splitVolume(v)
		if !namedVolumes[volName] {
			continue
		}

		if mountIsCarried(slice.Engine, mountPath) {
			continue
		}

		at := ""
		if mountPath != "" {
			at = " at " + mountPath
		}

		var message string
		if slice.Engine == "" {
			message = fmt.Sprintf(
				"service %q volume %s%s dropped — this service has no database or cache engine to carry it; add \"each copy needs its own storage of 10Gi\" to deploy the service as a StatefulSet with a 10Gi volume per copy, mounted at %s",
				name, volName, at, statefulLanding(mountPath))
		} else {
			message = fmt.Sprintf(
				"service %q volume %s%s not carried — a %s slice gets one volume, mounted at %s",
				name, volName, at, slice.Engine, engineMountPaths[slice.Engine])
		}

		warnings = append(warnings, Warning{
			Feature:  "volumes",
			Message:  message,
			Severity: "warn",
		})
	}

	// Environment.
	envs := parseEnvironment(svc.Environment)
	if len(envs) > 0 {
		slice.Env = envs
	}

	// Secrets.
	for _, s := range svc.Secrets {
		envVar := strings.ToUpper(strings.ReplaceAll(s.Source, "-", "_"))
		slice.Secrets = append(slice.Secrets, recipe.SecretRef{
			EnvVar:     envVar,
			SecretName: s.Source,
			Key:        s.Source,
		})
	}

	// Networks -> NetworkPolicy (basic mapping).
	svcNetworks := parseServiceNetworks(svc.Networks)
	if len(svcNetworks) > 0 {
		slice.NetworkPolicy = &recipe.NetworkPolicySpec{
			AllowFrom: svcNetworks,
		}
	}

	return slice, warnings
}

// convertNetworks produces warnings for compose networks that need attention.
func convertNetworks(networks map[string]composeNetworkConfig) []Warning {
	var warnings []Warning
	for name, net := range networks {
		if net.Internal {
			warnings = append(warnings, Warning{
				Feature:  "networks",
				Message:  fmt.Sprintf("internal network %q mapped to NetworkPolicy deny-all with allowFrom — verify rules", name),
				Severity: "info",
			})
		}
	}
	return warnings
}

// parseServiceNetworks extracts network names from the service networks field.
func parseServiceNetworks(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []interface{}:
		nets := make([]string, 0, len(v))
		for _, n := range v {
			if s, ok := n.(string); ok {
				nets = append(nets, s)
			}
		}
		return nets
	case map[string]interface{}:
		nets := make([]string, 0, len(v))
		for name := range v {
			nets = append(nets, name)
		}
		sort.Strings(nets)
		return nets
	}
	return nil
}

// parsePort extracts the container port and whether it is public from a port spec.
func parsePort(portSpec string) (int, bool) {
	portSpec = strings.TrimSpace(portSpec)
	portSpec = strings.Split(portSpec, "/")[0] // strip protocol

	if strings.Contains(portSpec, ":") {
		parts := strings.SplitN(portSpec, ":", 2)
		port := parsePortNumber(parts[1])
		return port, true
	}
	return parsePortNumber(portSpec), false
}

// parsePortNumber converts a port string to int, returning 0 on failure.
func parsePortNumber(s string) int {
	var port int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			break
		}
		port = port*10 + int(ch-'0')
	}
	return port
}

// splitVolume splits one compose short-syntax volume — [SOURCE:]TARGET[:MODE] —
// into its source and the container path it asks for.
//
// Splitting on every colon and keeping the first two fields leaves the MODE
// behind. A mode — "ro", "z", "cached", or a comma-separated run of them — says
// how the container sees the volume, never where; read as part of the path it
// would put "/var/lib/mysql:rw" in front of a reader as though it were a
// directory, and have the importer call the mount uncarried for not matching
// "/var/lib/mysql".
//
// A Windows source keeps its drive letter, so a bind mount from a Windows host
// is not read as a volume named "C".
//
// An anonymous volume that carries a mode and no source — "/data:ro" — reads as
// a source of "/data" and a target of "ro". No caller acts on it: a compose
// volume name cannot contain "/", so that source matches no declared volume.
func splitVolume(v string) (string, string) {
	fields := strings.Split(v, ":")
	if len(fields) > 1 && len(fields[0]) == 1 && strings.HasPrefix(fields[1], `\`) {
		fields = append([]string{fields[0] + ":" + fields[1]}, fields[2:]...)
	}

	if len(fields) > 1 {
		return fields[0], fields[1]
	}

	return fields[0], ""
}

// storageFor reports the size a service's engine shorthand should carry, or ""
// when it should carry none. The shorthand — "postgres 16, 10Gi" — is the only
// storage form the importer emits; the grammar's other one, "each copy needs its
// own storage of 10Gi", is left for the reader to add, which is what a dropped
// volume's warning asks them to do. Both import paths take their size from this
// function so they cannot come to different answers about the same service.
func storageFor(engine string, volumes []string, namedVolumes map[string]bool) string {
	switch engine {
	case "postgres", "mysql":
		return "10Gi"
	case "redis":
		// A cache is ephemeral unless compose gave it a volume, and asking for
		// storage it was never meant to have would provision a stray PVC.
		for _, v := range volumes {
			if volName, _ := splitVolume(v); namedVolumes[volName] {
				return "10Gi"
			}
		}
	}
	return ""
}

// engineMountPaths is where the compiler really mounts an engine slice's volume,
// mirroring the mount path plan.Build reads out of its own engine defaults. It
// takes no notice of the path compose asked for, so the two agree only by
// coincidence — TestEngineMountPathsMatchWhereTheCompilerMounts holds this table
// against the plan the compiler actually builds.
var engineMountPaths = map[string]string{ //nolint:gochecknoglobals // immutable lookup table
	"postgres": "/var/lib/postgresql/data",
	"mysql":    "/var/lib/mysql",
	"redis":    "/data",
}

// mountIsCarried reports whether the compiler will really mount a volume at the
// path compose named. A slice's storage is one PVC at the engine's own path, so
// a compose mount anywhere else — or on a slice with no engine to supply a path
// — is not carried, however plainly the compose file asked for it.
func mountIsCarried(engine, mountPath string) bool {
	canonical, known := engineMountPaths[engine]

	return known && mountPath == canonical
}

// statefulLanding names where "each copy needs its own storage" puts a volume,
// and the compose path it displaces. A slice with no engine has no engine path
// for the compiler to use, so that volume lands at /data whatever compose asked
// for — advice that did not say so would leave the app writing to a directory
// nothing is mounted on.
func statefulLanding(composePath string) string {
	if composePath == "" || composePath == "/data" {
		return "/data"
	}

	return "/data rather than " + composePath
}

// writeVolumes writes what a service's named volumes can carry into the recipe.
// A slice carries one volume at most — an engine slice's rides on its shorthand's
// size and lands at the engine's own path, and a slice with no engine has no
// shorthand for the importer to write a size on, only the per-copy directive the
// warning below asks the reader to add — so everything compose declared beyond
// that is named as uncarried instead of vanishing between the compose file and
// the cluster.
func writeVolumes(b *strings.Builder, engine string, volumes []string, namedVolumes map[string]bool) {
	for _, v := range volumes {
		volName, mountPath := splitVolume(v)
		if !namedVolumes[volName] {
			continue
		}

		at := ""
		if mountPath != "" {
			at = " at " + mountPath
		}

		switch {
		case mountIsCarried(engine, mountPath):
			fmt.Fprintf(b, "  # mount %s at %s\n", volName, mountPath)
		case engine != "":
			fmt.Fprintf(b,
				"  # WARNING: volume %s%s not carried — a %s slice gets one volume, mounted at %s\n",
				volName, at, engine, engineMountPaths[engine])
		default:
			// The remedy belongs in the file, as it does for a dropped health
			// check: this recipe outlives the import output, and the reader of it
			// is no longer looking at compose.
			fmt.Fprintf(b,
				"  # WARNING: volume %s%s dropped — this slice has no database or cache engine to carry it; add \"each copy needs its own storage of 10Gi\" to get a 10Gi volume per copy, mounted at %s\n",
				volName, at, statefulLanding(mountPath))
		}
	}
}

// parseEngineImage checks if an image string is a known database/cache engine
// and returns the engine name and version. Returns ("", "") if not a known engine.
func parseEngineImage(image string) (engine, version string) {
	// Split image into name and tag: "postgres:16" -> ("postgres", "16").
	parts := strings.SplitN(image, ":", 2)
	imageName := parts[0]
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}

	// Strip registry prefix if present (e.g., "docker.io/library/postgres").
	segments := strings.Split(imageName, "/")
	baseName := segments[len(segments)-1]

	eng, ok := knownEngines[baseName]
	if !ok {
		return "", ""
	}

	// Use the tag as the version. Strip Alpine/bookworm/etc. suffixes.
	version = tag
	for _, suffix := range []string{"-alpine", "-bookworm", "-bullseye", "-slim", "-jammy"} {
		version = strings.TrimSuffix(version, suffix)
	}
	if version == "latest" || version == "" {
		switch eng {
		case "postgres":
			version = "16"
		case "redis":
			version = "7"
		case "mysql":
			version = "8"
		}
	}

	return eng, version
}

// writePort writes a port directive to the builder.
func writePort(b *strings.Builder, portSpec string) {
	// Formats: "8080:80", "8080", "8080:80/udp"
	portSpec = strings.TrimSpace(portSpec)

	// Strip protocol suffix for now.
	portSpec = strings.Split(portSpec, "/")[0]

	if strings.Contains(portSpec, ":") {
		parts := strings.SplitN(portSpec, ":", 2)
		containerPort := parts[1]
		fmt.Fprintf(b, "  open to the public on port %s\n", containerPort)
	} else {
		fmt.Fprintf(b, "  on port %s\n", portSpec)
	}
}

// writeHealthcheck writes a health check directive, preceded by the port the check
// needs when no published port has already supplied one. publishedPort is the
// container port already written for this service, or 0 when there was none.
func writeHealthcheck(b *strings.Builder, hc *composeHealthcheck, publishedPort int) {
	path, urlPort := extractHealthCheck(hc)
	if path == "" {
		return
	}

	if publishedPort == 0 && urlPort == 0 {
		// mozza validate rejects a health check with no port to aim at, so explain
		// the gap in the recipe rather than write a line that cannot compile. The
		// remedy belongs here too: this file outlives the import output, and by now
		// the reader is looking at a recipe, not at compose.
		fmt.Fprintf(b,
			"  # WARNING: health check %s dropped — no published port and none in the check URL; add \"on port <n>\", then restore \"health check %s\"\n",
			path, path)

		return
	}

	if publishedPort == 0 {
		fmt.Fprintf(b, "  on port %d\n", urlPort)
	}
	fmt.Fprintf(b, "  health check %s\n", path)
}

// extractHealthCheck extracts the HTTP path and the target port from a compose
// healthcheck command such as ["CMD", "curl", "-f", "http://localhost:3000/health"].
// It returns ("", 0) when no part of the command holds a localhost URL with a path.
//
// The port matters as much as the path: a recipe's health check inherits the
// slice's port, so a check imported without one cannot be aimed and `mozza
// validate` rejects it. A zero port means the URL named none — callers decide what
// to do about it, because the importer will not invent one.
func extractHealthCheck(hc *composeHealthcheck) (string, int) {
	for _, part := range hc.Test {
		_, rest, found := strings.Cut(part, "http://localhost")
		if !found {
			continue
		}

		// rest is ":3000/health || exit 1", or "/health" when the URL omits the
		// port, so a port is present only directly after the colon.
		port := 0
		if afterColon, hasPort := strings.CutPrefix(rest, ":"); hasPort {
			port = parsePortNumber(afterColon)
		}

		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			continue
		}
		path := rest[slashIdx:]
		if spaceIdx := strings.IndexAny(path, " |"); spaceIdx > 0 {
			path = path[:spaceIdx]
		}

		return path, port
	}

	return "", 0
}

// parseDependsOn handles both list and map forms of depends_on.
func parseDependsOn(raw interface{}) []string {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []interface{}:
		deps := make([]string, 0, len(v))
		for _, d := range v {
			if s, ok := d.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	case map[string]interface{}:
		deps := make([]string, 0, len(v))
		for name := range v {
			deps = append(deps, name)
		}
		sort.Strings(deps)
		return deps
	}
	return nil
}

// parseEnvironment handles both map and list forms of environment.
func parseEnvironment(raw interface{}) map[string]string {
	if raw == nil {
		return nil
	}

	envs := make(map[string]string)

	switch v := raw.(type) {
	case map[string]interface{}:
		for key, val := range v {
			envs[key] = fmt.Sprintf("%v", val)
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				envs[parts[0]] = parts[1]
			}
		}
	}
	return envs
}

// reservedWords are .mozza keywords that cannot be used as section headers.
var reservedWords = map[string]bool{
	"app":       true,
	"namespace": true,
	"images":    true,
}

// capitalize returns the string with its first letter uppercased.
// If the name is a reserved .mozza keyword, it appends "-svc" to avoid conflicts.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	if reservedWords[strings.ToLower(s)] {
		s = s + "-svc"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sortedKeys returns the sorted keys from a map with string keys.
// It works with map[string]T for any T via type assertion through interface{}.
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
