package doctor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Collector gathers real environment signals by querying the Docker daemon
// and scanning host ports. It implements the SignalCollector interface.
type Collector struct {
	logger *slog.Logger
}

// NewCollector creates a Collector with the given logger. If logger is nil,
// a default logger is used.
func NewCollector(logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Collector{logger: logger}
}

// collectTimeout is the maximum time each subprocess check is allowed to run.
const collectTimeout = 5 * time.Second

// Collect gathers environment signals by checking Docker reachability,
// listing local images, scanning for used ports, and checking K8s permissions.
func (c *Collector) Collect(ctx context.Context) (*Signal, error) {
	sig := &Signal{}

	subCtx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	c.collectDocker(subCtx, sig)
	c.collectImages(subCtx, sig)
	c.collectPorts(subCtx, sig)
	c.collectK8sPermissions(subCtx, sig)

	return sig, nil
}

// collectK8sPermissions checks Kubernetes RBAC permissions using kubectl auth can-i.
func (c *Collector) collectK8sPermissions(ctx context.Context, sig *Signal) {
	// Quick reachability check.
	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
	if err := cmd.Run(); err != nil {
		sig.K8sReachable = false
		sig.K8sError = fmt.Errorf("collectK8sPermissions: %w", err)
		c.logger.Debug("kubernetes cluster unreachable", "error", err)
		return
	}
	sig.K8sReachable = true
	sig.K8sPermissions = make(map[string]bool)

	// Check key permissions.
	checks := []struct {
		resource string
		verb     string
	}{
		{"deployments", "create"},
		{"services", "create"},
		{"ingresses", "create"},
		{"persistentvolumeclaims", "create"},
		{"namespaces", "create"},
		{"secrets", "get"},
	}

	for _, check := range checks {
		key := check.resource + "/" + check.verb
		checkCmd := exec.CommandContext(ctx, "kubectl", "auth", "can-i", check.verb, check.resource)
		out, err := checkCmd.Output()
		if err != nil {
			sig.K8sPermissions[key] = false
		} else {
			sig.K8sPermissions[key] = strings.TrimSpace(string(out)) == "yes"
		}
	}
}

// collectDocker checks whether the Docker daemon is reachable by running
// "docker info".
func (c *Collector) collectDocker(ctx context.Context, sig *Signal) {
	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		sig.DockerReachable = false
		sig.DockerError = fmt.Errorf("collectDocker: %w", err)
		c.logger.Warn("docker daemon unreachable", "error", err)
		return
	}
	sig.DockerReachable = true
}

// collectImages lists locally cached Docker images by running
// "docker image ls --format {{.Repository}}:{{.Tag}}".
func (c *Collector) collectImages(ctx context.Context, sig *Signal) {
	cmd := exec.CommandContext(ctx, "docker", "image", "ls",
		"--format", "{{.Repository}}:{{.Tag}}")

	out, err := cmd.Output()
	if err != nil {
		c.logger.Warn("failed to list docker images", "error", err)
		return
	}

	sig.AvailableImages = parseLines(out)
}

// collectPorts scans for TCP ports in LISTEN state by running
// "lsof -i -P -n" and parsing the output.
func (c *Collector) collectPorts(ctx context.Context, sig *Signal) {
	cmd := exec.CommandContext(ctx, "lsof", "-i", "-P", "-n")

	out, err := cmd.Output()
	if err != nil {
		// lsof may not be available on all systems or may exit non-zero
		// when no matching files are found.
		c.logger.Debug("lsof unavailable or returned no results", "error", err)
		return
	}

	sig.UsedPorts = parseLSOFPorts(out)
}

// parseLines splits command output into non-empty trimmed lines.
func parseLines(data []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// parseLSOFPorts extracts unique TCP LISTEN ports from lsof output. Each
// output line is expected to have the format:
//
//	COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
//
// where NAME contains an address like "*:8080" or "127.0.0.1:3000".
func parseLSOFPorts(data []byte) []int {
	seen := make(map[int]bool)
	var ports []int

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "LISTEN") {
			continue
		}

		port, ok := extractPort(line)
		if !ok {
			continue
		}
		if !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}

	return ports
}

// extractPort parses the port number from the last field of a lsof line.
// The NAME field is expected to end with ":PORT" (e.g., "*:8080").
func extractPort(line string) (int, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, false
	}

	name := fields[len(fields)-1]
	idx := strings.LastIndex(name, ":")
	if idx < 0 || idx == len(name)-1 {
		return 0, false
	}

	port, err := strconv.Atoi(name[idx+1:])
	if err != nil {
		return 0, false
	}

	return port, true
}
