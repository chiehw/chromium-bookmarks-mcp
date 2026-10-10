VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -s -w -X 'github.com/chiehw/chromium-bookmarks-mcp/internal/version.Version=$(VERSION)'
GOFLAGS = CGO_ENABLED=0

.PHONY: test build release

test:
	go test ./...

build:
	mkdir -p bin
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o bin/chromium-bookmarks-mcp ./cmd/chromium-bookmarks-mcp

release:
	python3 scripts/build-release.py --version "$(VERSION)"
