package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gshepptech/mozza/internal/detect"
)

type detectRequest struct {
	RepoURL string `json:"repo_url,omitempty"`
	Dir     string `json:"dir,omitempty"`
}

type detectResponse struct {
	Framework  string            `json:"framework"`
	Language   string            `json:"language"`
	Confidence string            `json:"confidence"`
	Port       int               `json:"port"`
	BuildCmd   string            `json:"build_cmd,omitempty"`
	StartCmd   string            `json:"start_cmd,omitempty"`
	BaseImage  string            `json:"base_image"`
	HealthPath string            `json:"health_path"`
	Dockerfile string            `json:"dockerfile"`
	Recipe     string            `json:"recipe"`
	Details    map[string]string `json:"details,omitempty"`
}

type generateRequest struct {
	Framework   string            `json:"framework"`
	Language    string            `json:"language"`
	AppName     string            `json:"app_name"`
	Port        int               `json:"port"`
	UserChoices map[string]string `json:"user_choices"`
}

type generateResponse struct {
	Recipe     string `json:"recipe"`
	Dockerfile string `json:"dockerfile"`
}

// handleDetect runs framework detection on a local directory.
// POST /api/v1/detect
func (s *Server) handleDetect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req detectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		dir := req.Dir
		if dir == "" && req.RepoURL != "" {
			// Use project dir as fallback scan target.
			dir = s.cfg.ProjectDir
		}
		if dir == "" {
			Error(w, http.StatusBadRequest, "dir or repo_url is required")
			return
		}

		// Sanitize: must be absolute path and exist.
		absDir, err := filepath.Abs(dir)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid directory path")
			return
		}

		info, err := os.Stat(absDir)
		if err != nil || !info.IsDir() {
			Error(w, http.StatusBadRequest, "directory does not exist")
			return
		}

		results, err := detect.Scan(absDir)
		if err != nil {
			Error(w, http.StatusInternalServerError,
				fmt.Sprintf("detection failed: %v", err))
			return
		}

		if len(results) == 0 {
			JSON(w, http.StatusOK, detectResponse{
				Framework:  "unknown",
				Confidence: "low",
			})
			return
		}

		best := results[0]
		JSON(w, http.StatusOK, detectResponse{
			Framework:  best.Framework,
			Language:   best.Language,
			Confidence: string(best.Confidence),
			Port:       best.Port,
			BuildCmd:   best.BuildCmd,
			StartCmd:   best.StartCmd,
			BaseImage:  best.BaseImage,
			HealthPath: best.HealthPath,
			Dockerfile: best.Dockerfile,
			Recipe:     best.Recipe,
			Details:    best.Details,
		})
	}
}

// handleDetectGenerate generates a recipe and Dockerfile from detection + user choices.
// POST /api/v1/detect/generate
func (s *Server) handleDetectGenerate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Framework == "" || req.AppName == "" {
			Error(w, http.StatusBadRequest,
				"framework and app_name are required")
			return
		}

		port := req.Port
		if port == 0 {
			port = 8080
		}

		recipe := buildRecipeFromChoices(
			req.AppName, req.Framework, req.Language, port,
			req.UserChoices,
		)
		dockerfile := buildDockerfileFromChoices(
			req.Framework, req.Language, port, req.UserChoices,
		)

		JSON(w, http.StatusOK, generateResponse{
			Recipe:     recipe,
			Dockerfile: dockerfile,
		})
	}
}

// buildRecipeFromChoices generates a .mozza recipe incorporating user answers.
func buildRecipeFromChoices(
	appName, framework, language string,
	port int,
	choices map[string]string,
) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("App: %s\n\n", appName))
	sb.WriteString(fmt.Sprintf("# Framework: %s (%s)\n\n", framework, language))

	// Main app slice.
	sb.WriteString("App:\n")
	sb.WriteString(fmt.Sprintf("  from image %s:latest\n", appName))
	sb.WriteString(fmt.Sprintf("  open to the public on port %d\n", port))
	sb.WriteString("  health check /health\n")
	sb.WriteString("  run 1 copy\n")
	sb.WriteString("  limit cpu 500m memory 256Mi\n")
	sb.WriteString("\n")

	// Database slice from user choice.
	if db, ok := choices["database"]; ok && db != "" && db != "none" {
		dbPort := 5432
		dbImage := "postgres:16-alpine"
		switch db {
		case "mysql":
			dbPort = 3306
			dbImage = "mysql:8"
		case "mongodb":
			dbPort = 27017
			dbImage = "mongo:7"
		}
		sb.WriteString("Database:\n")
		sb.WriteString(fmt.Sprintf("  from image %s\n", dbImage))
		sb.WriteString(fmt.Sprintf("  on port %d\n", dbPort))
		sb.WriteString("  store 10Gi\n")
		sb.WriteString("\n")
	}

	// Background worker from user choice.
	if worker, ok := choices["worker"]; ok && worker == "yes" {
		workerName := "worker"
		if wn, ok := choices["worker_name"]; ok && wn != "" {
			workerName = wn
		}
		sb.WriteString(fmt.Sprintf("%s:\n",
			strings.ToUpper(workerName[:1])+workerName[1:]))
		sb.WriteString(fmt.Sprintf("  from image %s:latest\n", appName))
		sb.WriteString("  run 1 copy\n")
		sb.WriteString("  limit cpu 250m memory 128Mi\n")
		sb.WriteString("\n")
	}

	// Cache from user choice.
	if cache, ok := choices["cache"]; ok && cache == "yes" {
		sb.WriteString("Cache:\n")
		sb.WriteString("  redis\n")
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// buildDockerfileFromChoices generates a Dockerfile for the detected framework.
func buildDockerfileFromChoices(
	framework, language string,
	port int,
	choices map[string]string,
) string {
	switch framework {
	case "nextjs":
		return buildNextDockerfile(port, choices)
	case "django":
		return buildPythonDockerfile("django", port, choices)
	case "flask":
		return buildPythonDockerfile("flask", port, choices)
	case "rails":
		return buildRailsDockerfile(port, choices)
	case "laravel":
		return buildLaravelDockerfile(port, choices)
	default:
		return buildGenericDockerfile(language, port)
	}
}

func buildNextDockerfile(port int, _ map[string]string) string {
	return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE %d
CMD ["node", "server.js"]
`, port)
}

func buildPythonDockerfile(framework string, port int, _ map[string]string) string {
	cmd := `["gunicorn", "app:app", "--bind", "0.0.0.0:` +
		fmt.Sprintf("%d", port) + `"]`
	if framework == "django" {
		cmd = `["gunicorn", "config.wsgi:application", "--bind", "0.0.0.0:` +
			fmt.Sprintf("%d", port) + `"]`
	}
	return fmt.Sprintf(`FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE %d
CMD %s
`, port, cmd)
}

func buildRailsDockerfile(port int, _ map[string]string) string {
	return fmt.Sprintf(`FROM ruby:3.3-slim
WORKDIR /app
COPY Gemfile Gemfile.lock ./
RUN bundle install --without development test
COPY . .
RUN bundle exec rake assets:precompile
EXPOSE %d
CMD ["bundle", "exec", "rails", "server", "-b", "0.0.0.0", "-p", "%d"]
`, port, port)
}

func buildLaravelDockerfile(port int, choices map[string]string) string {
	octane := ""
	if v, ok := choices["octane"]; ok && v == "yes" {
		octane = "\nRUN php artisan octane:install --server=swoole"
	}
	return fmt.Sprintf(`FROM php:8.3-cli
WORKDIR /app
COPY composer.json composer.lock ./
RUN composer install --no-dev --optimize-autoloader
COPY . .%s
EXPOSE %d
CMD ["php", "artisan", "serve", "--host=0.0.0.0", "--port=%d"]
`, octane, port, port)
}

func buildGenericDockerfile(language string, port int) string {
	switch language {
	case "go":
		return fmt.Sprintf(`FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/app .

FROM alpine:3.20
COPY --from=builder /bin/app /bin/app
EXPOSE %d
CMD ["/bin/app"]
`, port)
	default:
		return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --production
COPY . .
EXPOSE %d
CMD ["npm", "start"]
`, port)
	}
}
