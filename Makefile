.PHONY: all build test clean install uninstall version

# Get version from git tag or use date-based version
VERSION := $(shell git describe --tags --always 2>/dev/null || date +"%Y%m%d")
BUILD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X github.com/deca-org/deca/cmd.Version=$(VERSION) -X github.com/deca-org/deca/cmd.Build=$(BUILD)

# Default target
all: build

# Build the binary
build:
	@echo "Building deca $(VERSION) ($(BUILD))..."
	go build -ldflags "$(LDFLAGS)" -o deca .

# Build with debug info
debug:
	go build -ldflags "$(LDFLAGS)" -gcflags="all=-N -l" -o deca .

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

# Create release binaries
release:
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			echo "Building for $$os/$$arch..."; \
			GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "dist/deca-$$os-$$arch" .; \
		done; \
	done

# Setup development environment
setup:
	go mod download
	go get github.com/schollz/progressbar/v3
	go get github.com/fatih/color
	go get github.com/mattn/go-isatty
