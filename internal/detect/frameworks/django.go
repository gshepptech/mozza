package frameworks

import (
	"path/filepath"
	"strings"

	"github.com/gshepptech/mozza/internal/detect"
)

func init() {
	detect.Register(&djangoDetector{})
}

type djangoDetector struct{}

func (d *djangoDetector) Name() string { return "django" }

func (d *djangoDetector) Detect(dir string) *detect.Result {
	depFiles := findPythonDepFiles(dir)
	if len(depFiles) == 0 {
		return nil
	}

	if !hasDjangoDep(depFiles) {
		return nil
	}

	r := &detect.Result{
		Framework:  "django",
		Language:   "python",
		Confidence: detect.ConfidenceMedium,
		Port:       8000,
		BuildCmd:   "pip install -r requirements.txt",
		StartCmd:   "python manage.py runserver 0.0.0.0:8000",
		BaseImage:  "python:3.12-slim",
		HealthPath: "/health/",
		Details:    make(map[string]string),
	}

	if fileExists(filepath.Join(dir, "manage.py")) {
		r.Confidence = detect.ConfidenceHigh
		r.Details["manage_py"] = "found"
	}

	detectWSGIServer(r, depFiles)

	if fileExists(filepath.Join(dir, "staticfiles")) ||
		fileExists(filepath.Join(dir, "static")) {
		r.Details["static_files"] = "found"
	}

	r.Dockerfile = djangoDockerfile(r)
	r.Recipe = detect.GenerateRecipeText("myapp", r)

	return r
}

// findPythonDepFiles returns the paths of Python dependency files found in dir.
func findPythonDepFiles(dir string) []string {
	candidates := []string{"requirements.txt", "pyproject.toml", "Pipfile"}
	var found []string
	for _, f := range candidates {
		p := filepath.Join(dir, f)
		if fileExists(p) {
			found = append(found, p)
		}
	}
	return found
}

// hasDjangoDep checks whether any of the dependency files contain Django.
func hasDjangoDep(depFiles []string) bool {
	for _, f := range depFiles {
		content := readFileContent(f)
		if containsDependency(content, "django") || containsDependency(content, "Django") {
			return true
		}
	}
	return false
}

// detectWSGIServer detects gunicorn or uvicorn in dependencies and updates the result.
func detectWSGIServer(r *detect.Result, depFiles []string) {
	var sb strings.Builder
	for _, f := range depFiles {
		sb.WriteString(readFileContent(f))
	}
	deps := sb.String()

	if containsDependency(deps, "gunicorn") {
		r.StartCmd = "gunicorn config.wsgi:application --bind 0.0.0.0:8000"
		r.Details["wsgi_server"] = "gunicorn"
	} else if containsDependency(deps, "uvicorn") {
		r.StartCmd = "uvicorn config.asgi:application --host 0.0.0.0 --port 8000"
		r.Details["wsgi_server"] = "uvicorn"
	}
}

// containsDependency checks if a dependency name appears in a requirements-like string.
func containsDependency(content, name string) bool {
	lower := strings.ToLower(content)
	nameLower := strings.ToLower(name)
	for _, line := range strings.Split(lower, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, nameLower) ||
			strings.Contains(trimmed, "\""+nameLower+"\"") ||
			strings.Contains(trimmed, "'"+nameLower+"'") {
			return true
		}
	}
	return false
}

func djangoDockerfile(r *detect.Result) string {
	collectStatic := ""
	if r.Details["static_files"] == "found" {
		collectStatic = "RUN python manage.py collectstatic --noinput\n"
	}

	return `FROM python:3.12-slim AS builder
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM python:3.12-slim
WORKDIR /app
RUN addgroup --system --gid 1001 django && \
    adduser --system --uid 1001 --gid 1001 django
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY . .
` + collectStatic + `USER 1001
EXPOSE 8000
CMD ["` + strings.Join(strings.Fields(r.StartCmd), "\", \"") + `"]
`
}
