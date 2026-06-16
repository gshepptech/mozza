package frameworks

import (
	"path/filepath"
	"strings"

	"github.com/gshepptech/mozza/internal/detect"
)

func init() {
	detect.Register(&railsDetector{})
}

type railsDetector struct{}

func (d *railsDetector) Name() string { return "rails" }

func (d *railsDetector) Detect(dir string) *detect.Result {
	gemfilePath := filepath.Join(dir, "Gemfile")
	if !fileExists(gemfilePath) {
		return nil
	}

	content := readFileContent(gemfilePath)
	if !strings.Contains(content, "rails") {
		return nil
	}

	r := &detect.Result{
		Framework:  "rails",
		Language:   "ruby",
		Confidence: detect.ConfidenceMedium,
		Port:       3000,
		BuildCmd:   "bundle install",
		StartCmd:   "bundle exec rails server -b 0.0.0.0 -p 3000",
		BaseImage:  "ruby:3.3-slim",
		HealthPath: "/up",
		Details:    make(map[string]string),
	}

	// Check for Rakefile or config.ru to raise confidence.
	if fileExists(filepath.Join(dir, "config.ru")) {
		r.Confidence = detect.ConfidenceHigh
		r.Details["config_ru"] = "found"
	} else if fileExists(filepath.Join(dir, "Rakefile")) {
		r.Confidence = detect.ConfidenceHigh
	}

	// Check for web server in Gemfile.lock.
	lockContent := readFileContent(filepath.Join(dir, "Gemfile.lock"))
	if strings.Contains(lockContent, "puma") {
		r.StartCmd = "bundle exec puma -C config/puma.rb"
		r.Details["web_server"] = "puma"
	}

	// Check database adapter from config/database.yml.
	dbConfig := readFileContent(filepath.Join(dir, "config", "database.yml"))
	if dbConfig != "" {
		switch {
		case strings.Contains(dbConfig, "postgresql"):
			r.Details["database"] = "postgresql"
		case strings.Contains(dbConfig, "mysql"):
			r.Details["database"] = "mysql"
		case strings.Contains(dbConfig, "sqlite"):
			r.Details["database"] = "sqlite"
		}
	}

	r.Dockerfile = railsDockerfile(r)
	r.Recipe = detect.GenerateRecipeText("myapp", r)

	return r
}

func railsDockerfile(r *detect.Result) string {
	return `FROM ruby:3.3-slim AS builder
WORKDIR /app
RUN apt-get update -qq && \
    apt-get install --no-install-recommends -y build-essential libpq-dev && \
    rm -rf /var/lib/apt/lists/*
COPY Gemfile Gemfile.lock ./
RUN bundle config set --local deployment true && \
    bundle config set --local without development test && \
    bundle install

FROM ruby:3.3-slim
WORKDIR /app
RUN apt-get update -qq && \
    apt-get install --no-install-recommends -y libpq5 && \
    rm -rf /var/lib/apt/lists/* && \
    addgroup --system --gid 1001 rails && \
    adduser --system --uid 1001 --gid 1001 rails
COPY --from=builder /app/vendor/bundle /app/vendor/bundle
COPY --from=builder /usr/local/bundle /usr/local/bundle
COPY . .
RUN bundle exec rails assets:precompile 2>/dev/null || true
USER 1001
EXPOSE 3000
CMD ["` + strings.Join(strings.Fields(r.StartCmd), "\", \"") + `"]
`
}
