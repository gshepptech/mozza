package frameworks

import (
	"path/filepath"
	"strings"

	"github.com/gshepptech/mozza/internal/detect"
)

func init() {
	detect.Register(&nextjsDetector{})
}

type nextjsDetector struct{}

func (d *nextjsDetector) Name() string { return "nextjs" }

func (d *nextjsDetector) Detect(dir string) *detect.Result {
	pkgPath := filepath.Join(dir, "package.json")
	if !fileExists(pkgPath) {
		return nil
	}

	pkg, err := readPackageJSON(pkgPath)
	if err != nil {
		return nil
	}

	if !pkg.hasDependency("next") {
		return nil
	}

	r := &detect.Result{
		Framework:  "nextjs",
		Language:   "javascript",
		Confidence: detect.ConfidenceMedium,
		Port:       3000,
		BuildCmd:   "npm run build",
		StartCmd:   "npm start",
		BaseImage:  "node:20-alpine",
		HealthPath: "/api/health",
		Details:    make(map[string]string),
	}

	applyNextConfig(dir, r)

	// Detect package manager.
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		r.Details["package_manager"] = "yarn"
		r.BuildCmd = "yarn build"
		r.StartCmd = "yarn start"
	} else if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		r.Details["package_manager"] = "pnpm"
		r.BuildCmd = "pnpm build"
		r.StartCmd = "pnpm start"
	}

	r.Recipe = detect.GenerateRecipeText("myapp", r)

	return r
}

// applyNextConfig detects output mode from next.config and sets the Dockerfile.
func applyNextConfig(dir string, r *detect.Result) {
	configPath := findNextConfig(dir)
	if configPath != "" {
		r.Confidence = detect.ConfidenceHigh
		r.Details["config_file"] = filepath.Base(configPath)

		content := readFileContent(configPath)
		if strings.Contains(content, `output: 'standalone'`) ||
			strings.Contains(content, `output: "standalone"`) {
			r.Details["output_mode"] = "standalone"
			r.Dockerfile = nextjsStandaloneDockerfile()
		} else if strings.Contains(content, `output: 'export'`) ||
			strings.Contains(content, `output: "export"`) {
			r.Details["output_mode"] = "export"
			r.Dockerfile = nextjsExportDockerfile()
			r.StartCmd = ""
			r.BaseImage = "nginx:alpine"
			r.Port = 80
		}
	}

	if r.Dockerfile == "" {
		r.Dockerfile = nextjsDefaultDockerfile()
	}
}

// findNextConfig looks for next.config.{js,mjs,ts} in the directory.
func findNextConfig(dir string) string {
	for _, name := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func nextjsStandaloneDockerfile() string {
	return `FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs
COPY --from=builder /app/public ./public
COPY --from=builder --chown=1001:1001 /app/.next/standalone ./
COPY --from=builder --chown=1001:1001 /app/.next/static ./.next/static
USER 1001
EXPOSE 3000
ENV PORT=3000
CMD ["node", "server.js"]
`
}

func nextjsExportDockerfile() string {
	return `FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/out /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`
}

func nextjsDefaultDockerfile() string {
	return `FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/public ./public
USER 1001
EXPOSE 3000
ENV PORT=3000
CMD ["npm", "start"]
`
}
