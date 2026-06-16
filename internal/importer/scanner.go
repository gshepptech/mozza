package importer

import (
	"fmt"
	"strings"
)

// ScanResult describes what was found in a repository.
type ScanResult struct {
	RepoURL     string           `json:"repo_url"`
	RepoName    string           `json:"repo_name"`
	Description string           `json:"description"`
	Sources     []DetectedSource `json:"sources"`
	Generated   *GeneratedRecipe `json:"generated,omitempty"`
	Warnings    []string         `json:"warnings"`
}

// DetectedSource is something deployable found in the repo.
type DetectedSource struct {
	Type     string `json:"type"` // "dockerfile", "compose", "helm", "k8s-manifests"
	Path     string `json:"path"`
	Priority int    `json:"priority"` // lower number = higher priority
}

// GeneratedRecipe is the auto-generated recipe from a repo scan.
type GeneratedRecipe struct {
	Source            string `json:"source"`                       // .mozza content
	Method            string `json:"method"`                       // "from-compose", "from-helm", "from-dockerfile"
	Editable          bool   `json:"editable"`                     // always true
	NeedsBuild        bool   `json:"needs_build"`                  // true when repo has no pre-built image
	BuildInstructions string `json:"build_instructions,omitempty"` // shell steps to build & push the image
}

// ScanOptions configures the Scan operation.
type ScanOptions struct {
	// Token is an optional GitHub personal access token for private repos.
	// It is never stored or logged server-side.
	Token string `json:"-"`
}

// Scan fetches a GitHub repository and generates a .mozza recipe from what it finds.
// Pass nil for opts to use defaults (no authentication).
func Scan(repoURL string, opts *ScanOptions) (*ScanResult, error) {
	if opts == nil {
		opts = &ScanOptions{}
	}

	owner, repo, err := ParseGitHubURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	name, description, err := FetchRepoMeta(owner, repo, opts.Token)
	if err != nil {
		return nil, fmt.Errorf("fetching repo metadata: %w", err)
	}

	files, err := ListRootFiles(owner, repo, opts.Token)
	if err != nil {
		return nil, fmt.Errorf("listing repo files: %w", err)
	}

	result := &ScanResult{
		RepoURL:     repoURL,
		RepoName:    name,
		Description: description,
	}

	detectSources(result, files)

	if len(result.Sources) == 0 {
		result.Warnings = append(result.Warnings, "No deployable sources found in repository")
		return result, nil
	}

	if err := generateRecipe(result, owner, repo, opts.Token); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Could not generate recipe: %v", err))
	}

	return result, nil
}

// detectSources scans a file listing for known deployable sources.
func detectSources(result *ScanResult, files []string) {
	for _, f := range files {
		name := strings.TrimSuffix(f, "/")
		lower := strings.ToLower(name)

		switch {
		case lower == "docker-compose.yml" || lower == "docker-compose.yaml":
			result.Sources = append(result.Sources, DetectedSource{
				Type:     "compose",
				Path:     name,
				Priority: 1,
			})
		case lower == "chart.yaml":
			result.Sources = append(result.Sources, DetectedSource{
				Type:     "helm",
				Path:     name,
				Priority: 2,
			})
		case lower == "charts" || lower == "chart":
			result.Sources = append(result.Sources, DetectedSource{
				Type:     "helm",
				Path:     name + "/",
				Priority: 2,
			})
		case lower == "k8s" || lower == "manifests" || lower == "deploy":
			result.Sources = append(result.Sources, DetectedSource{
				Type:     "k8s-manifests",
				Path:     name + "/",
				Priority: 3,
			})
		case lower == "dockerfile":
			result.Sources = append(result.Sources, DetectedSource{
				Type:     "dockerfile",
				Path:     name,
				Priority: 4,
			})
		}
	}
}

// generateRecipe uses the highest-priority detected source to produce a recipe.
func generateRecipe(result *ScanResult, owner, repo, token string) error {
	if len(result.Sources) == 0 {
		return fmt.Errorf("no sources to generate from")
	}

	// Find highest priority (lowest number).
	best := result.Sources[0]
	for _, s := range result.Sources[1:] {
		if s.Priority < best.Priority {
			best = s
		}
	}

	switch best.Type {
	case "compose":
		return generateFromCompose(result, owner, repo, best.Path, token)
	case "helm":
		return generateFromHelm(result, owner, repo, best.Path, token)
	case "dockerfile":
		return generateFromDockerfile(result, owner, repo, best.Path, token)
	case "k8s-manifests":
		result.Warnings = append(result.Warnings, "Kubernetes manifest conversion is not yet supported")
		return nil
	default:
		return fmt.Errorf("unknown source type: %s", best.Type)
	}
}

// generateFromCompose fetches and converts a docker-compose file.
func generateFromCompose(result *ScanResult, owner, repo, path, token string) error {
	content, err := FetchFile(owner, repo, path, token)
	if err != nil {
		return fmt.Errorf("fetching compose file: %w", err)
	}

	recipe, err := ComposeToRecipe(content)
	if err != nil {
		return fmt.Errorf("converting compose file: %w", err)
	}

	result.Generated = &GeneratedRecipe{
		Source:   recipe,
		Method:   "from-compose",
		Editable: true,
	}
	result.Warnings = append(result.Warnings, "Recipe generated from docker-compose.yml — review before deploying")
	return nil
}

// generateFromHelm fetches and converts a Helm chart.
func generateFromHelm(result *ScanResult, owner, repo, path, token string) error {
	// Determine values.yaml path.
	valuesPath := "values.yaml"
	if strings.HasSuffix(path, "/") {
		valuesPath = path + "values.yaml"
	} else if path == "Chart.yaml" {
		valuesPath = "values.yaml"
	}

	content, err := FetchFile(owner, repo, valuesPath, token)
	if err != nil {
		// Fallback: generate a basic recipe if we cannot read values.yaml.
		result.Generated = &GeneratedRecipe{
			Source:   fmt.Sprintf("App: %s\n\n%s:\n  from image %s:latest\n  on port 8080\n", repo, capitalize(repo), repo),
			Method:   "from-helm",
			Editable: true,
		}
		result.Warnings = append(result.Warnings, "Could not read values.yaml — generated basic recipe")
		return nil
	}

	recipe, err := HelmToRecipe(content, repo)
	if err != nil {
		return fmt.Errorf("converting helm chart: %w", err)
	}

	result.Generated = &GeneratedRecipe{
		Source:   recipe,
		Method:   "from-helm",
		Editable: true,
	}
	result.Warnings = append(result.Warnings, "Recipe generated from Helm values.yaml — review before deploying")
	return nil
}

// generateFromDockerfile fetches and converts a Dockerfile.
func generateFromDockerfile(result *ScanResult, owner, repo, path, token string) error {
	content, err := FetchFile(owner, repo, path, token)
	if err != nil {
		return fmt.Errorf("fetching Dockerfile: %w", err)
	}

	recipe, err := DockerfileToRecipe(content, repo)
	if err != nil {
		return fmt.Errorf("converting Dockerfile: %w", err)
	}

	ghcrImage := fmt.Sprintf("ghcr.io/%s/%s:latest", owner, repo)

	// Check whether a pre-built image already exists on GHCR.
	if CheckGHCR(owner, repo) {
		result.Generated = &GeneratedRecipe{
			Source:     strings.Replace(recipe, repo+":latest", ghcrImage, 1),
			Method:     "from-dockerfile",
			Editable:   true,
			NeedsBuild: false,
		}
		result.Warnings = append(result.Warnings,
			"Pre-built image found on ghcr.io — recipe updated to use it. Review before deploying.")
		return nil
	}

	buildInstructions := fmt.Sprintf(
		"1. Clone: git clone https://github.com/%s/%s\n"+
			"2. Build: docker build -t %s .\n"+
			"3. Push: docker push %s",
		owner, repo, ghcrImage, ghcrImage,
	)

	result.Generated = &GeneratedRecipe{
		Source:            recipe,
		Method:            "from-dockerfile",
		Editable:          true,
		NeedsBuild:        true,
		BuildInstructions: buildInstructions,
	}
	result.Warnings = append(result.Warnings,
		"Recipe generated from Dockerfile — this repo needs to be built first. Push the image to a registry, then deploy.")
	return nil
}
