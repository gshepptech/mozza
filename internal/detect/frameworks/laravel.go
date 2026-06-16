package frameworks

import (
	"encoding/json"
	"path/filepath"

	"github.com/gshepptech/mozza/internal/detect"
)

func init() {
	detect.Register(&laravelDetector{})
}

type laravelDetector struct{}

func (d *laravelDetector) Name() string { return "laravel" }

func (d *laravelDetector) Detect(dir string) *detect.Result {
	composerPath := filepath.Join(dir, "composer.json")
	if !fileExists(composerPath) {
		return nil
	}

	data := readFileContent(composerPath)
	if data == "" {
		return nil
	}

	var composer struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal([]byte(data), &composer); err != nil {
		return nil
	}

	if _, ok := composer.Require["laravel/framework"]; !ok {
		return nil
	}

	r := &detect.Result{
		Framework:  "laravel",
		Language:   "php",
		Confidence: detect.ConfidenceMedium,
		Port:       8000,
		BuildCmd:   "composer install --no-dev --optimize-autoloader",
		StartCmd:   "php artisan serve --host=0.0.0.0 --port=8000",
		BaseImage:  "php:8.3-fpm-alpine",
		HealthPath: "/up",
		Details:    make(map[string]string),
	}

	// Check for artisan file to raise confidence.
	if fileExists(filepath.Join(dir, "artisan")) {
		r.Confidence = detect.ConfidenceHigh
		r.Details["artisan"] = "found"
	}

	// Check for Laravel Octane.
	if _, ok := composer.Require["laravel/octane"]; ok {
		r.Details["octane"] = "enabled"
		r.StartCmd = "php artisan octane:start --host=0.0.0.0 --port=8000"
	}

	r.Dockerfile = laravelDockerfile(r)
	r.Recipe = detect.GenerateRecipeText("myapp", r)

	return r
}

func laravelDockerfile(r *detect.Result) string {
	return `FROM composer:2 AS deps
WORKDIR /app
COPY composer.json composer.lock ./
RUN composer install --no-dev --no-scripts --no-autoloader

FROM php:8.3-fpm-alpine
RUN apk add --no-cache nginx && \
    docker-php-ext-install pdo pdo_mysql opcache
WORKDIR /app
COPY --from=deps /app/vendor ./vendor
COPY . .
RUN composer dump-autoload --optimize && \
    php artisan config:cache && \
    php artisan route:cache && \
    chown -R www-data:www-data storage bootstrap/cache
EXPOSE 8000
CMD ["php", "artisan", "serve", "--host=0.0.0.0", "--port=8000"]
`
}
