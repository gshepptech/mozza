.PHONY: build test vet lint fmt check ui clean updates

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG     := github.com/gshepptech/mozza/internal/version
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

build:
	go build -ldflags="$(LDFLAGS)" -o bin/mozza ./cmd/mozza
test:
	go test -race -count=1 ./...
vet:
	go vet ./...
lint:
	golangci-lint run ./...
fmt:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)
check: fmt vet test
ui:
	cd ui && npm ci && npm run build
clean:
	rm -rf bin/ coverage.out
updates:
	go list -u -m all
