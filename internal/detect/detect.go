// Package detect provides framework auto-detection for project directories.
// It scans marker files (package.json, go.mod, requirements.txt, etc.) and
// identifies the framework, language, and optimal deployment configuration.
package detect

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gshepptech/mozza/internal/recipe"
)

// Confidence indicates how certain the detection is.
type Confidence string

const (
	// ConfidenceHigh means a framework-specific config file was found.
	ConfidenceHigh Confidence = "high"

	// ConfidenceMedium means a dependency was found but no config file.
	ConfidenceMedium Confidence = "medium"

	// ConfidenceLow means only generic project structure was found.
	ConfidenceLow Confidence = "low"
)

// Result holds the output of a framework scan.
type Result struct {
	// Framework is the detected framework name (e.g. "nextjs", "django").
	Framework string `json:"framework"`

	// Language is the programming language (e.g. "javascript", "python", "go").
	Language string `json:"language"`

	// Confidence indicates detection certainty.
	Confidence Confidence `json:"confidence"`

	// Port is the default port for the framework.
	Port int `json:"port"`

	// BuildCmd is the command to build the project.
	BuildCmd string `json:"build_cmd,omitempty"`

	// StartCmd is the command to start the application.
	StartCmd string `json:"start_cmd,omitempty"`

	// BaseImage is the recommended Docker base image.
	BaseImage string `json:"base_image"`

	// HealthPath is the recommended health-check endpoint.
	HealthPath string `json:"health_path"`

	// Dockerfile is the generated Dockerfile content.
	Dockerfile string `json:"dockerfile"`

	// Recipe is the generated .mozza recipe content.
	Recipe string `json:"recipe"`

	// Details holds framework-specific customization notes.
	Details map[string]string `json:"details,omitempty"`
}

// Detector is the interface that framework detectors implement.
type Detector interface {
	// Name returns the framework identifier (e.g. "nextjs", "django").
	Name() string

	// Detect checks whether the directory contains this framework and returns
	// a Result if found. Returns nil if the framework is not detected.
	Detect(dir string) *Result
}

// registry holds all registered detectors in priority order.
var registry []Detector

// Register adds a detector to the registry. Detectors are evaluated in
// registration order; register more specific detectors first.
func Register(d Detector) {
	registry = append(registry, d)
}

// Scan examines the project directory and returns detection results sorted
// by confidence (high first). Returns an error only for filesystem issues.
func Scan(dir string) ([]Result, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("detect.Scan: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("detect.Scan: %s is not a directory", dir)
	}

	slog.Debug("scanning directory for frameworks", "dir", dir)

	var results []Result
	for _, d := range registry {
		r := d.Detect(dir)
		if r != nil {
			slog.Info("detected framework",
				"framework", r.Framework,
				"language", r.Language,
				"confidence", r.Confidence,
			)
			results = append(results, *r)
		}
	}

	// Sort by confidence: high > medium > low.
	sort.Slice(results, func(i, j int) bool {
		return confidenceRank(results[i].Confidence) > confidenceRank(results[j].Confidence)
	})

	return results, nil
}

// ScanBest examines the project directory and returns the highest-confidence
// result, or nil if no framework was detected.
func ScanBest(dir string) (*Result, error) {
	results, err := Scan(dir)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// GenerateRecipe builds a recipe.Recipe from a detection result.
func GenerateRecipe(appName string, r *Result) *recipe.Recipe {
	rec := &recipe.Recipe{
		Name: appName,
		Slices: []recipe.Slice{
			{
				Name:        "app",
				Kind:        "web",
				Image:       fmt.Sprintf("%s:latest", appName),
				Port:        r.Port,
				Public:      true,
				Health:      r.HealthPath,
				Replicas:    1,
				CPULimit:    "500m",
				MemoryLimit: "256Mi",
			},
		},
	}

	return rec
}

// GenerateRecipeText builds the .mozza recipe text from a detection result.
func GenerateRecipeText(appName string, r *Result) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("App: %s\n\n", appName))
	sb.WriteString(fmt.Sprintf("# Auto-detected: %s (%s confidence)\n\n", r.Framework, r.Confidence))
	sb.WriteString(fmt.Sprintf("App:\n"))
	sb.WriteString(fmt.Sprintf("  from image %s:latest\n", appName))
	sb.WriteString(fmt.Sprintf("  open to the public on port %d\n", r.Port))
	sb.WriteString(fmt.Sprintf("  health check %s\n", r.HealthPath))
	sb.WriteString("  run 1 copy\n")
	sb.WriteString("  limit cpu 500m memory 256Mi\n")

	return sb.String()
}

// confidenceRank converts confidence to a numeric rank for sorting.
func confidenceRank(c Confidence) int {
	switch c {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads a file and returns its content as a string.
// Returns empty string on any error.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// readJSON reads a JSON file into the target struct.
// Returns an error if the file cannot be read or parsed.
func readJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readJSON: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("readJSON: %w", err)
	}
	return nil
}

// containsLine checks if any line in content contains the substring.
func containsLine(content, substr string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

// joinPath is a convenience wrapper for filepath.Join.
func joinPath(parts ...string) string {
	return filepath.Join(parts...)
}
