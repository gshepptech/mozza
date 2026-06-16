package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gshepptech/mozza/internal/plan"
)

// httpClient is the HTTP client used for registry requests (overridable in tests).
var httpClient = &http.Client{Timeout: 10 * time.Second}

// ValidateImages checks that every image referenced in the plan exists in its
// registry. Returns a hard error for missing/unauthorized images, and logs
// warnings for architecture mismatches (which will run under emulation).
func ValidateImages(ctx context.Context, p *plan.AppPlan) error {
	creds := loadDockerCredentials()

	var errors []string
	for _, s := range p.Slices {
		if s.Image == "" {
			continue
		}
		if err := checkImage(ctx, s.Image, creds); err != nil {
			msg := err.Error()
			// Arch mismatches are warnings, not hard failures — the image
			// exists but will run under QEMU emulation.
			if strings.Contains(msg, "emulation") || strings.Contains(msg, "no linux/") {
				slog.Warn("architecture mismatch", "slice", s.Name, "image", s.Image, "detail", msg)
				continue
			}
			errors = append(errors, fmt.Sprintf("  %s: image %q — %s", s.Name, s.Image, msg))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("image validation failed:\n%s\nCheck that these images exist and you have access to them", strings.Join(errors, "\n"))
	}
	return nil
}

// manifestAccept is the Accept header for registry manifest requests.
// Includes fat manifest (manifest list) for multi-arch detection.
const manifestAccept = "application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json"

// checkImage queries the Docker Registry HTTP API V2 to verify an image exists
// and is available for the current architecture.
func checkImage(ctx context.Context, image string, creds map[string]registryAuth) error {
	registry, repo, tag := parseImageRef(image)

	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repo, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("could not be checked: %w", err)
	}

	req.Header.Set("Accept", manifestAccept)

	// Add auth if we have credentials for this registry.
	if auth, ok := creds[registry]; ok {
		req.Header.Set("Authorization", "Basic "+auth.encoded())
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("registry unreachable: %w", err)
	}
	defer resp.Body.Close()

	// Docker Hub returns 401 for public images — fetch bearer token and retry.
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("Www-Authenticate")
		token := fetchBearerToken(ctx, authHeader)
		if token == "" {
			return fmt.Errorf("not authorized — check your Docker login for %s", registry)
		}
		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req2.Header.Set("Accept", manifestAccept)
		req2.Header.Set("Authorization", "Bearer "+token)
		resp2, err2 := httpClient.Do(req2)
		if err2 != nil {
			return fmt.Errorf("registry unreachable on retry: %w", err2)
		}
		resp.Body.Close()
		resp = resp2
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return checkArchCompat(resp, registry)
	case http.StatusNotFound:
		return fmt.Errorf("not found in registry %s", registry)
	default:
		return fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
	}
}

// checkArchCompat reads the manifest response and warns if the image doesn't
// support the current architecture.
func checkArchCompat(resp *http.Response, registry string) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil // can't read body, skip arch check
	}

	ct := resp.Header.Get("Content-Type")
	hostArch := runtime.GOARCH
	// Container images always target Linux, even when the host is macOS/Windows
	// (Docker Desktop and K8s run Linux VMs under the hood).
	hostOS := "linux"

	// Fat manifest (manifest list) — check if our arch is in the list.
	if strings.Contains(ct, "manifest.list") || strings.Contains(ct, "image.index") {
		var index struct {
			Manifests []struct {
				Platform struct {
					Architecture string `json:"architecture"`
					OS           string `json:"os"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if err := json.Unmarshal(body, &index); err != nil {
			return nil // can't parse, skip
		}

		for _, m := range index.Manifests {
			if m.Platform.Architecture == hostArch && m.Platform.OS == hostOS {
				return nil
			}
		}

		// List available architectures in the error.
		var archs []string
		for _, m := range index.Manifests {
			archs = append(archs, m.Platform.OS+"/"+m.Platform.Architecture)
		}
		return fmt.Errorf("no %s/%s image available (found: %s)", hostOS, hostArch, strings.Join(archs, ", "))
	}

	// Single manifest — check config for architecture.
	if strings.Contains(ct, "manifest.v2") || strings.Contains(ct, "image.manifest") {
		var manifest struct {
			Config struct {
				Digest string `json:"digest"`
			} `json:"config"`
		}
		if err := json.Unmarshal(body, &manifest); err != nil || manifest.Config.Digest == "" {
			return nil // can't determine arch, skip
		}

		// Fetch the config blob to get the architecture.
		_, repo, _ := parseImageRef("")
		_ = repo // already have registry
		configURL := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry,
			extractRepo(resp.Request), manifest.Config.Digest)

		configReq, _ := http.NewRequestWithContext(resp.Request.Context(), http.MethodGet, configURL, nil)
		if auth := resp.Request.Header.Get("Authorization"); auth != "" {
			configReq.Header.Set("Authorization", auth)
		}

		configResp, err := httpClient.Do(configReq)
		if err != nil || configResp.StatusCode != http.StatusOK {
			return nil // can't fetch config, skip
		}
		defer configResp.Body.Close()

		var config struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		}
		if err := json.NewDecoder(configResp.Body).Decode(&config); err != nil {
			return nil
		}

		if config.Architecture != "" && config.Architecture != hostArch {
			return fmt.Errorf("image is %s/%s but host is %s/%s — will run under emulation (slow, may crash)",
				config.OS, config.Architecture, hostOS, hostArch)
		}
	}

	return nil
}

// extractRepo gets the repository path from the request URL.
func extractRepo(req *http.Request) string {
	// URL is like /v2/hobbyfarm/gargantua/manifests/v3.3.5
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) >= 5 {
		return strings.Join(parts[2:len(parts)-2], "/")
	}
	return ""
}

// fetchBearerToken parses a Www-Authenticate header and fetches a bearer token.
// Returns empty string if the header is not a bearer challenge or token fetch fails.
func fetchBearerToken(ctx context.Context, header string) string {
	// Format: Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}

	params := make(map[string]string)
	for _, part := range strings.Split(header[7:], ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = strings.Trim(kv[1], "\"")
		}
	}

	realm := params["realm"]
	if realm == "" {
		return ""
	}

	tokenURL := realm
	sep := "?"
	if svc := params["service"]; svc != "" {
		tokenURL += sep + "service=" + svc
		sep = "&"
	}
	if scope := params["scope"]; scope != "" {
		tokenURL += sep + "scope=" + scope
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return ""
	}

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return result.Token
}

// parseImageRef splits a Docker image reference into registry, repository, and tag.
func parseImageRef(image string) (registry, repo, tag string) {
	// Split tag/digest.
	tag = "latest"
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		// Make sure this isn't a port number (contains no /).
		after := image[idx+1:]
		if !strings.Contains(after, "/") {
			tag = after
			image = image[:idx]
		}
	}
	if idx := strings.LastIndex(image, "@"); idx > 0 {
		tag = image[idx+1:]
		image = image[:idx]
	}

	// Determine registry vs repo.
	parts := strings.SplitN(image, "/", 2)
	if len(parts) == 1 {
		// e.g., "nginx" → Docker Hub library image.
		return "registry-1.docker.io", "library/" + parts[0], tag
	}

	first := parts[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		// e.g., "ghcr.io/org/repo" or "localhost:5000/img"
		return first, parts[1], tag
	}

	// e.g., "myorg/myimage" → Docker Hub.
	return "registry-1.docker.io", image, tag
}

// registryAuth holds credentials for a container registry.
type registryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// encoded returns the base64-encoded "user:pass" for HTTP Basic auth.
func (a registryAuth) encoded() string {
	if a.Auth != "" {
		return a.Auth
	}
	return base64.StdEncoding.EncodeToString([]byte(a.Username + ":" + a.Password))
}

// dockerConfig represents the structure of ~/.docker/config.json.
type dockerConfig struct {
	Auths map[string]registryAuth `json:"auths"`
}

// loadDockerCredentials reads ~/.docker/config.json for registry auth.
func loadDockerCredentials() map[string]registryAuth {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(home, ".docker", "config.json"))
	if err != nil {
		return nil
	}

	var cfg dockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	return cfg.Auths
}

// ValidateSecrets checks that all secrets referenced in the plan exist in the
// target namespace and contain the expected keys.
func ValidateSecrets(ctx context.Context, cs kubernetes.Interface, namespace string, p *plan.AppPlan) error {
	var problems []string
	for _, s := range p.Slices {
		for _, ref := range s.Secrets {
			secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, ref.SecretName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					problems = append(problems, fmt.Sprintf(
						"  %s: secret %q does not exist in namespace %q", s.Name, ref.SecretName, namespace))
					continue
				}
				return fmt.Errorf("ValidateSecrets: get %q: %w", ref.SecretName, err)
			}

			if _, ok := secret.Data[ref.Key]; !ok {
				problems = append(problems, fmt.Sprintf(
					"  %s: secret %q exists but has no key %q", s.Name, ref.SecretName, ref.Key))
			}
		}

		if s.PullSecret != "" {
			_, err := cs.CoreV1().Secrets(namespace).Get(ctx, s.PullSecret, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					problems = append(problems, fmt.Sprintf(
						"  %s: pull secret %q does not exist in namespace %q", s.Name, s.PullSecret, namespace))
					continue
				}
				return fmt.Errorf("ValidateSecrets: get pull secret %q: %w", s.PullSecret, err)
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("secret validation failed:\n%s\nCreate these secrets before deploying", strings.Join(problems, "\n"))
	}
	return nil
}

// EnsureNamespace checks if the namespace exists and creates it if missing.
// Returns true if the namespace was created.
func EnsureNamespace(ctx context.Context, cs kubernetes.Interface, namespace string) (bool, error) {
	_, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return false, nil
	}

	if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("EnsureNamespace: check %q: %w", namespace, err)
	}

	slog.Info("creating namespace", "namespace", namespace)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err = cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, fmt.Errorf("EnsureNamespace: create %q: %w", namespace, err)
	}
	return true, nil
}
