package importer

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// HelmToRecipe converts a Helm chart values.yaml into a .mozza recipe string.
// chartName is used as the application and service name in the generated recipe.
func HelmToRecipe(valuesYAML string, chartName string) (string, error) {
	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYAML), &values); err != nil {
		return "", fmt.Errorf("parsing values.yaml: %w", err)
	}

	if chartName == "" {
		chartName = "app"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "App: %s\n\n", chartName)
	fmt.Fprintf(&b, "%s:\n", capitalize(chartName))

	// Extract image.
	imageRepo, imageTag := extractHelmImage(values)
	if imageRepo != "" {
		tag := imageTag
		if tag == "" {
			tag = "latest"
		}
		fmt.Fprintf(&b, "  from image %s:%s\n", imageRepo, tag)
	} else {
		fmt.Fprintf(&b, "  # WARNING: could not detect image from values.yaml\n")
		fmt.Fprintf(&b, "  from image %s:latest\n", chartName)
	}

	// Extract port.
	port := extractHelmPort(values)
	if port != "" {
		fmt.Fprintf(&b, "  on port %s\n", port)
	}

	// Check for ingress.
	if ingressEnabled(values) {
		// If ingress is enabled, the service should be public.
		if port != "" {
			// Rewrite last line to be public.
			content := b.String()
			content = strings.Replace(content,
				fmt.Sprintf("  on port %s\n", port),
				fmt.Sprintf("  open to the public on port %s\n", port), 1)
			b.Reset()
			b.WriteString(content)
		}
	}

	// Extract replicas.
	replicas := extractHelmReplicas(values)
	if replicas > 1 {
		fmt.Fprintf(&b, "  run %d copies\n", replicas)
	}

	return b.String(), nil
}

// extractHelmImage looks for image.repository and image.tag in values.
func extractHelmImage(values map[string]interface{}) (repo, tag string) {
	imageRaw, ok := values["image"]
	if !ok {
		return "", ""
	}

	imageMap, ok := imageRaw.(map[string]interface{})
	if !ok {
		return "", ""
	}

	if r, ok := imageMap["repository"].(string); ok {
		repo = r
	}
	if t, ok := imageMap["tag"].(string); ok {
		tag = t
	}
	return repo, tag
}

// extractHelmPort looks for service.port or containerPort in values.
func extractHelmPort(values map[string]interface{}) string {
	// Try service.port first.
	if svcRaw, ok := values["service"]; ok {
		if svcMap, ok := svcRaw.(map[string]interface{}); ok {
			if port, ok := svcMap["port"]; ok {
				return fmt.Sprintf("%v", port)
			}
		}
	}

	// Try containerPort.
	if port, ok := values["containerPort"]; ok {
		return fmt.Sprintf("%v", port)
	}

	return ""
}

// extractHelmReplicas looks for replicaCount in values.
func extractHelmReplicas(values map[string]interface{}) int {
	if r, ok := values["replicaCount"]; ok {
		switch v := r.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return 0
}

// ingressEnabled checks if ingress.enabled is true in values.
func ingressEnabled(values map[string]interface{}) bool {
	ingressRaw, ok := values["ingress"]
	if !ok {
		return false
	}
	ingressMap, ok := ingressRaw.(map[string]interface{})
	if !ok {
		return false
	}
	enabled, ok := ingressMap["enabled"].(bool)
	return ok && enabled
}
