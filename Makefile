.PHONY: all build test clean install uninstall version static release musl-release

# Get version from git tag or use date-based version
VERSION := $(shell git describe --tags --always 2>/dev/null || date +"%Y%m%d")
BUILD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X github.com/deca-org/deca/cmd.Version=$(VERSION) -X github.com/deca-org/deca/cmd.Build=$(BUILD)
STATIC_LDFLAGS := $(LDFLAGS) -linkmode=external -extldflags=-static

# Default target
all: build

# Build the binary
build:
	@echo "Building deca $(VERSION) ($(BUILD))..."
	go build -ldflags "$(LDFLAGS)" -o deca .

# Build with debug info
debug:
	go build -ldflags "$(LDFLAGS)" -gcflags="all=-N -l" -o deca .

# Static build for maximum compatibility (no glibc dependency)
# Note: Requires musl-gcc for full static linking
# Ubuntu/Debian: sudo apt install musl-tools
static:
	@echo "Building static deca $(VERSION) ($(BUILD))..."
	@if ! command -v musl-gcc >/dev/null 2>&1; then \
		echo "Error: musl-gcc not found. Install with: sudo apt install musl-tools"; \
		exit 1; \
	fi
	CC=musl-gcc CGO_ENABLED=1 go build -ldflags "$(STATIC_LDFLAGS)" -o deca .

# Run tests
test:
	go test ./...

# Install to system
install:
	go install -ldflags "$(LDFLAGS)" .

# Show version
version:
	@echo "deca version: $(VERSION)"
	@echo "build: $(BUILD)"
	@echo "go: $(shell go version)"

# Clean build artifacts
clean:
	rm -f deca

# Create release binaries (dynamic linked)
release:
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			echo "Building for $$os/$$arch..."; \
			GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "dist/deca-$$os-$$arch" .; \
		done; \
	done

# Create musl-based static release binaries (for Linux, maximum compatibility)
musl-release:
	@mkdir -p dist
	@echo "Note: musl-release requires musl-gcc (sudo apt install musl-tools)"
	@for arch in amd64 arm64; do \
		echo "Building static for linux/$$arch..."; \
		CC=musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=$$arch go build -ldflags "$(STATIC_LDFLAGS)" -o "dist/deca-linux-$$arch-static" .; \
	done

# Setup development environment
setup:
	go mod download
	go get github.com/schollz/progressbar/v3
	go get github.com/fatih/color
	go get github.com/mattn/go-isatty
