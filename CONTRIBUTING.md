# Contributing to Mozza

Thanks for wanting to contribute. Here is how to get started.

## Development Setup

### Prerequisites

- Go 1.24 or later
- golangci-lint
- Node.js 20+ (for UI development)
- Docker (for local testing)
- Make

### Getting the Code

```bash
git clone https://github.com/gshepptech/mozza.git
cd mozza
```

### Building

```bash
make build     # Compiles to bin/mozza
make test      # Runs tests with -race
make lint      # Runs golangci-lint
make check     # All of the above plus security checks
```

### Running Locally

```bash
# Start the web dashboard
./bin/mozza serve

# In another terminal, deploy a test app
./bin/mozza init test-app
./bin/mozza up
```

### UI Development

The React UI lives in `ui/`. To develop with hot reload:

```bash
cd ui
npm install
npm run dev
```

The dev server proxies API requests to the Go backend on port 8080.

When you are done, build the production UI bundle:

```bash
cd ui
npm run build
```

The built assets are embedded into the Go binary via `go:embed`.

## Pull Request Process

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Run `make check` and fix any issues
4. Write tests for new functionality
5. Keep commits atomic -- one logical change per commit
6. Open a pull request with a clear description of what and why

### Commit Messages

Use this format:

```
package: short description in imperative mood

Optional longer explanation of why the change is needed.
```

Examples:

```
parser: handle multi-line environment variables
handler: return 404 instead of 500 for missing deployments
recipe: add validation for duplicate slice names
```

### What Makes a Good PR

- Solves one problem
- Includes tests
- Passes `make check`
- Has a description that explains the motivation, not just the mechanics
- Keeps functions under 40 lines
- Handles errors explicitly -- no silent discards

## Coding Standards

### Go

- **Logging**: Use `slog` only. No `log`, no `fmt.Print` in production code.
- **Errors**: Wrap with `fmt.Errorf("context: %w", err)`. Never ignore errors silently.
- **Testing**: Use `testify` for assertions. Prefer table-driven tests.
- **Functions**: Keep them under 40 lines and cyclomatic complexity under 10.
- **Documentation**: All exported types, functions, and methods need doc comments.

### UI (TypeScript/React)

- Functional components with hooks
- No class components
- Inline styles (project convention, not Tailwind)

## How to Add a Recipe Template

Recipe templates live in the template catalog. To add one:

1. Create the recipe file with all required slices
2. Add metadata (name, description, category, icon)
3. Test it locally with `mozza up`
4. Add it to the template catalog in the dashboard
5. Open a PR

## How to Add a CLI Command

CLI commands use Cobra. To add one:

1. Create a new file in `cmd/` (e.g., `cmd/mycommand.go`)
2. Define the Cobra command with `Use`, `Short`, `Long`, and `RunE`
3. Register it in the root command
4. Add tests
5. Update the CLI reference in README.md

## Reporting Bugs

Use the [bug report template](.github/ISSUE_TEMPLATE/bug-report.yml) on GitHub.
Include: what you expected, what happened, steps to reproduce, and your
environment (OS, Go version, Mozza version).

## Suggesting Features

Use the [feature request template](.github/ISSUE_TEMPLATE/feature-request.yml).
Describe the problem you are trying to solve, not just the solution you want.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Be
respectful, be constructive, be welcoming.
