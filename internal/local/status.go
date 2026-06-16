package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
)

// ContainerStatus represents the runtime state of a single Docker Compose container.
type ContainerStatus struct {
	// Name is the container name assigned by Docker Compose.
	Name string
	// Service is the Compose service name.
	Service string
	// State is the container state (running, exited, etc.).
	State string
	// Image is the container image reference.
	Image string
	// Ports is the published port mapping string.
	Ports string
}

// composePS mirrors the JSON output of "docker compose ps --format json".
type composePS struct {
	Name       string `json:"Name"`
	Service    string `json:"Service"`
	State      string `json:"State"`
	Image      string `json:"Image"`
	Publishers []struct {
		URL           string `json:"URL"`
		TargetPort    int    `json:"TargetPort"`
		PublishedPort int    `json:"PublishedPort"`
		Protocol      string `json:"Protocol"`
	} `json:"Publishers"`
}

// StatusQuery queries Docker Compose for container status.
type StatusQuery struct {
	// ProjectDir is the working directory where docker-compose.yml lives.
	ProjectDir string
}

// Run executes "docker compose ps --format json" and returns structured container statuses.
func (q *StatusQuery) Run(ctx context.Context) ([]ContainerStatus, error) {
	raw, err := q.execComposePS(ctx)
	if err != nil {
		return nil, fmt.Errorf("Run: %w", err)
	}

	return parseComposePS(raw)
}

// execComposePS runs the docker compose ps command and returns the raw output.
func (q *StatusQuery) execComposePS(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "json")
	if q.ProjectDir != "" {
		cmd.Dir = q.ProjectDir
	}

	slog.Debug("running docker compose ps", "dir", q.ProjectDir)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("execComposePS: %w", err)
	}

	return out, nil
}

// parseComposePS parses the JSON output from docker compose ps into ContainerStatus slices.
func parseComposePS(raw []byte) ([]ContainerStatus, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var containers []composePS
	if err := json.Unmarshal(raw, &containers); err != nil {
		// Docker compose may output one JSON object per line instead of an array.
		containers, err = parseJSONLines(raw)
		if err != nil {
			return nil, fmt.Errorf("parseComposePS: %w", err)
		}
	}

	return toContainerStatuses(containers), nil
}

// parseJSONLines handles docker compose output where each line is a separate JSON object.
func parseJSONLines(raw []byte) ([]composePS, error) {
	var containers []composePS
	dec := json.NewDecoder(bytes.NewReader(raw))
	for dec.More() {
		var c composePS
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("parseJSONLines: %w", err)
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// toContainerStatuses converts parsed compose output to ContainerStatus values.
func toContainerStatuses(containers []composePS) []ContainerStatus {
	statuses := make([]ContainerStatus, 0, len(containers))
	for _, c := range containers {
		statuses = append(statuses, ContainerStatus{
			Name:    c.Name,
			Service: c.Service,
			State:   c.State,
			Image:   c.Image,
			Ports:   formatPorts(c.Publishers),
		})
	}
	return statuses
}

// formatPorts builds a human-readable port mapping string from publishers.
func formatPorts(publishers []struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}) string {
	if len(publishers) == 0 {
		return ""
	}

	seen := make(map[string]struct{})
	var result string
	for _, p := range publishers {
		if p.PublishedPort == 0 {
			continue
		}
		entry := fmt.Sprintf("%d->%d/%s", p.PublishedPort, p.TargetPort, p.Protocol)
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		if result != "" {
			result += ", "
		}
		result += entry
	}
	return result
}
