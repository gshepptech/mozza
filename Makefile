.PHONY: lint test security check score complexity build clean health updates

lint:
	golangci-lint run ./...
test:
	go test -race -coverprofile=coverage.out -covermode=atomic -count=1 ./...
security:
	@bash .claude/scripts/go-security.sh
check: lint test security
	@echo "✅ All passed"
score: check
	@bash .claude/scripts/quality-score.sh
complexity:
	@bash .claude/scripts/go-complexity.sh
build:
	go build -ldflags="-s -w" -o bin/ ./cmd/...
clean:
	rm -f coverage.out && rm -rf bin/ && go clean ./...
health:
	@bash .claude/scripts/health-check.sh
updates:
	@bash .claude/scripts/check-updates.sh
