.PHONY: build build-lean build-all build-all-lean test clean install help

# Variables
VERSION ?= 0.5.0
BINARY_NAME := web-recap
DIST_DIR := dist
BIN_DIR := bin
GO := go
GOFLAGS := -ldflags="-s -w"

# Build tags for optional output backends
# Use TAGS=nomongo,nomysql,nopq to exclude them from the build
TAGS ?=

# Platform targets
LINUX_AMD64 := $(DIST_DIR)/$(BINARY_NAME)-linux-amd64
LINUX_ARM64 := $(DIST_DIR)/$(BINARY_NAME)-linux-arm64
DARWIN_AMD64 := $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64
DARWIN_ARM64 := $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64
WINDOWS_AMD64 := $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe

# Lean platform targets (no Mongo/MySQL/Postgres)
LEAN_TAGS := nomongo,nomysql,nopq,noingest
LINUX_AMD64_LEAN := $(DIST_DIR)/$(BINARY_NAME)-linux-amd64-lean
LINUX_ARM64_LEAN := $(DIST_DIR)/$(BINARY_NAME)-linux-arm64-lean
DARWIN_AMD64_LEAN := $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64-lean
DARWIN_ARM64_LEAN := $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64-lean
WINDOWS_AMD64_LEAN := $(DIST_DIR)/$(BINARY_NAME)-windows-amd64-lean.exe

help:
	@echo "web-recap build targets:"
	@echo "  make build              - Build for current platform (all backends)"
	@echo "  make build-lean         - Build for current platform (SQLite output only, ~46% smaller)"
	@echo "  make build-all          - Build for all platforms (all backends)"
	@echo "  make build-all-lean     - Build for all platforms (SQLite output only)"
	@echo "  make test               - Run tests"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make install            - Install binary to GOBIN"
	@echo "  make help               - Show this help message"
	@echo ""
	@echo "Optional output backend tags (default: all included):"
	@echo "  nomongo  - exclude MongoDB ingest support"
	@echo "  nomysql  - exclude MySQL ingest support"
	@echo "  nopq     - exclude PostgreSQL ingest support"
	@echo "  Example: go build -tags nomongo,nomysql,nopq ..."

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/web-recap

build-lean:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -tags $(LEAN_TAGS) -o $(BIN_DIR)/$(BINARY_NAME)-lean ./cmd/web-recap

build-all: $(LINUX_AMD64) $(LINUX_ARM64) $(DARWIN_AMD64) $(DARWIN_ARM64) $(WINDOWS_AMD64)
	@echo "✓ Built all platforms (full)"

build-all-lean: $(LINUX_AMD64_LEAN) $(LINUX_ARM64_LEAN) $(DARWIN_AMD64_LEAN) $(DARWIN_ARM64_LEAN) $(WINDOWS_AMD64_LEAN)
	@echo "✓ Built all platforms (lean)"

$(LINUX_AMD64):
	@mkdir -p $(DIST_DIR)
	@echo "Building Linux AMD64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $@ ./cmd/web-recap

$(LINUX_ARM64):
	@mkdir -p $(DIST_DIR)
	@echo "Building Linux ARM64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $@ ./cmd/web-recap

$(DARWIN_AMD64):
	@mkdir -p $(DIST_DIR)
	@echo "Building macOS Intel..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $@ ./cmd/web-recap

$(DARWIN_ARM64):
	@mkdir -p $(DIST_DIR)
	@echo "Building macOS ARM64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $@ ./cmd/web-recap

$(WINDOWS_AMD64):
	@mkdir -p $(DIST_DIR)
	@echo "Building Windows AMD64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $@ ./cmd/web-recap

$(LINUX_AMD64_LEAN):
	@mkdir -p $(DIST_DIR)
	@echo "Building Linux AMD64 (lean)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -tags $(LEAN_TAGS) -o $@ ./cmd/web-recap

$(LINUX_ARM64_LEAN):
	@mkdir -p $(DIST_DIR)
	@echo "Building Linux ARM64 (lean)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -tags $(LEAN_TAGS) -o $@ ./cmd/web-recap

$(DARWIN_AMD64_LEAN):
	@mkdir -p $(DIST_DIR)
	@echo "Building macOS Intel (lean)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -tags $(LEAN_TAGS) -o $@ ./cmd/web-recap

$(DARWIN_ARM64_LEAN):
	@mkdir -p $(DIST_DIR)
	@echo "Building macOS ARM64 (lean)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -tags $(LEAN_TAGS) -o $@ ./cmd/web-recap

$(WINDOWS_AMD64_LEAN):
	@mkdir -p $(DIST_DIR)
	@echo "Building Windows AMD64 (lean)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -tags $(LEAN_TAGS) -o $@ ./cmd/web-recap

test:
	$(GO) test ./...

test-verbose:
	$(GO) test -v ./...

test-coverage:
	$(GO) test -cover ./...

clean:
	@echo "Cleaning build artifacts..."
	$(GO) clean
	rm -rf $(DIST_DIR)
	rm -rf $(BIN_DIR)
	rm -f $(BINARY_NAME)

install: build
	@echo "Installing web-recap..."
	$(GO) install ./cmd/web-recap

deps:
	$(GO) mod download
	$(GO) mod verify

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run ./...

vet:
	$(GO) vet ./...

.PHONY: build build-lean build-all build-all-lean test test-verbose test-coverage clean install deps fmt lint vet help
