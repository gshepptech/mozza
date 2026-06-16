package importer

import (
	"fmt"
	"strings"
)

// DockerfileToRecipe converts a Dockerfile into a .mozza recipe string.
// repoName is used as the application and service name. ownerRepo should
// be in "owner/repo" format for constructing the GHCR image reference.
func DockerfileToRecipe(dockerfile string, repoName string) (string, error) {
	if strings.TrimSpace(dockerfile) == "" {
		return "", fmt.Errorf("empty Dockerfile")
	}

	if repoName == "" {
		repoName = "app"
	}

	port := extractExposePort(dockerfile)
	if port == "" {
		port = "8080"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "App: %s\n\n", repoName)
	fmt.Fprintf(&b, "%s:\n", capitalize(repoName))
	fmt.Fprintf(&b, "  from image %s:latest\n", repoName)
	fmt.Fprintf(&b, "  open to the public on port %s\n", port)
	fmt.Fprintf(&b, "  health check /healthz\n")
	fmt.Fprintf(&b, "  run 1 copy\n")

	return b.String(), nil
}

// extractExposePort scans a Dockerfile for EXPOSE directives and returns
// the first port found. Returns an empty string if no EXPOSE is found.
func extractExposePort(dockerfile string) string {
	for _, line := range strings.Split(dockerfile, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(trimmed), "EXPOSE") {
			continue
		}

		// "EXPOSE 8080" or "EXPOSE 8080/tcp"
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			continue
		}

		portStr := parts[1]
		// Strip protocol suffix.
		portStr = strings.Split(portStr, "/")[0]
		return portStr
	}
	return ""
}
