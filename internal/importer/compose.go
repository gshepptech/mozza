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
			// Add default storage for database engines.
			if engineName == "postgres" || engineName == "mysql" {
				line += ", 10Gi"
			}
			b.WriteString(line + "\n")
		} else {
			fmt.Fprintf(&b, "  from image %s\n", image)
		}

		// Ports.
		for _, p := range svc.Ports {
			writePort(&b, p)
		}

		// Health check.
		if svc.Healthcheck != nil {
			writeHealthcheck(&b, svc.Healthcheck)
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

		// Named volumes -> storage.
		writeVolumes(&b, svc.Volumes, namedVolumes)

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
		if engineName == "postgres" || engineName == "mysql" {
			slice.Storage = "10Gi"
		}
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

	// Health check.
	if svc.Healthcheck != nil {
		slice.Health = extractHealthPath(svc.Healthcheck)
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

	// Named volumes -> storage.
	for _, v := range svc.Volumes {
		volName, mountPath := splitVolume(v)
		if namedVolumes[volName] {
			if slice.Storage == "" {
				slice.Storage = "10Gi"
			}
			if mountPath != "" {
				slice.Mounts = append(slice.Mounts, recipe.MountSpec{
					Type:   "pvc",
					Source: volName,
					Target: mountPath,
				})
			}
		}
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

// splitVolume splits "name:/path" into volume name and mount path.
func splitVolume(v string) (string, string) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return v, ""
}

// writeVolumes writes storage and mount directives for named volumes.
func writeVolumes(b *strings.Builder, volumes []string, namedVolumes map[string]bool) {
	for _, v := range volumes {
		volName, mountPath := splitVolume(v)
		if namedVolumes[volName] {
			fmt.Fprintf(b, "  storage 10Gi\n")
			if mountPath != "" {
				fmt.Fprintf(b, "  # mount %s at %s\n", volName, mountPath)
			}
			return // only emit one storage directive per service
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

// writeHealthcheck extracts a health check path from a compose healthcheck.
func writeHealthcheck(b *strings.Builder, hc *composeHealthcheck) {
	path := extractHealthPath(hc)
	if path != "" {
		fmt.Fprintf(b, "  health check %s\n", path)
	}
}

// extractHealthPath extracts an HTTP path from a compose healthcheck.
func extractHealthPath(hc *composeHealthcheck) string {
	if len(hc.Test) == 0 {
		return ""
	}
	for _, part := range hc.Test {
		if strings.Contains(part, "http://localhost") {
			idx := strings.Index(part, "http://localhost")
			rest := part[idx+len("http://localhost"):]
			if colonIdx := strings.Index(rest, "/"); colonIdx >= 0 {
				path := rest[colonIdx:]
				if spaceIdx := strings.IndexAny(path, " |"); spaceIdx > 0 {
					path = path[:spaceIdx]
				}
				return path
			}
		}
	}
	return ""
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
