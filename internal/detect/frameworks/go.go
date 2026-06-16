package frameworks

import (
	"path/filepath"
	"strings"

	"github.com/gshepptech/mozza/internal/detect"
)

func init() {
	detect.Register(&goDetector{})
}

type goDetector struct{}

func (d *goDetector) Name() string { return "go" }

func (d *goDetector) Detect(dir string) *detect.Result {
	goModPath := filepath.Join(dir, "go.mod")
	if !fileExists(goModPath) {
		return nil
	}

	content := readFileContent(goModPath)
	if content == "" {
		return nil
	}

	r := &detect.Result{
		Framework:  "go",
		Language:   "go",
		Confidence: detect.ConfidenceHigh, // go.mod is definitive
		Port:       8080,
		BuildCmd:   "go build -o /app ./...",
		StartCmd:   "/app",
		BaseImage:  "golang:1.24-alpine",
		HealthPath: "/healthz",
		Details:    make(map[string]string),
	}

	// Extract module name.
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "module ") {
			r.Details["module"] = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}

	// Check for CGO usage — if found, cannot use static linking.
	usesCGO := false
	goFiles := findGoFiles(dir)
	for _, f := range goFiles {
		src := readFileContent(f)
		if strings.Contains(src, `import "C"`) || strings.Contains(src, "// #cgo") {
			usesCGO = true
			break
		}
	}

	if usesCGO {
		r.Details["cgo"] = "enabled"
		r.BuildCmd = "go build -o /app ./..."
		r.Dockerfile = goCGODockerfile()
	} else {
		r.Details["cgo"] = "disabled"
		r.BuildCmd = "CGO_ENABLED=0 go build -ldflags='-s -w' -o /app ./..."
		r.Dockerfile = goStaticDockerfile()
	}

	// Detect main package location.
	if fileExists(filepath.Join(dir, "cmd")) {
		r.Details["cmd_dir"] = "found"
	}

	r.Recipe = detect.GenerateRecipeText("myapp", r)

	return r
}

// findGoFiles returns Go source files in the root directory only (not recursive).
func findGoFiles(dir string) []string {
	entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil
	}
	// Also check cmd/ directory.
	cmdFiles, _ := filepath.Glob(filepath.Join(dir, "cmd", "**", "*.go"))
	return append(entries, cmdFiles...)
}

func goStaticDockerfile() string {
	return `FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags='-s -w' -o /app ./...

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app /app
USER 65534
EXPOSE 8080
ENTRYPOINT ["/app"]
`
}

func goCGODockerfile() string {
	return `FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app ./...

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 1001 appuser
COPY --from=builder /app /app
USER 1001
EXPOSE 8080
ENTRYPOINT ["/app"]
`
}
