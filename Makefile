# Build variables
BINARIES=provisioner workspacectl templatectl jobctl
BIN_DIR=./bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION ?= $(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS = -ldflags "\
	-X provisioner/pkg/version.Version=${VERSION} \
	-X provisioner/pkg/version.GitCommit=${GIT_COMMIT} \
	-X provisioner/pkg/version.BuildDate=${BUILD_DATE} \
	-w -s"

# Go build flags
BUILD_FLAGS = -a -installsuffix cgo

# Platforms for cross-compilation
PLATFORMS = linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64

# Default target
.PHONY: all
all: clean build-install build

# Generate install script from templates
.PHONY: build-install
build-install:
	@echo "Generating install script from templates..."
	./scripts/build-install.sh

# Build all binaries
.PHONY: build
build: $(BIN_DIR)
	@echo "Building all binaries ${VERSION}..."
	@for binary in $(BINARIES); do \
		echo "Building $$binary..."; \
		CGO_ENABLED=0 go build ${BUILD_FLAGS} ${LDFLAGS} -o ${BIN_DIR}/$$binary ./cmd/$$binary; \
	done

# Build for development (with race detection)
.PHONY: build-dev
build-dev: $(BIN_DIR)
	@echo "Building all binaries for development..."
	@for binary in $(BINARIES); do \
		echo "Building $$binary with race detection..."; \
		go build -race ${LDFLAGS} -o ${BIN_DIR}/$$binary ./cmd/$$binary; \
	done

# Cross-compile for all platforms
.PHONY: build-all
build-all: clean dist
	@echo "Cross-compiling for all platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		for binary in $(BINARIES); do \
			echo "Building $$binary for $$platform..."; \
			CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build ${BUILD_FLAGS} ${LDFLAGS} \
				-o dist/$$binary-$$GOOS-$$GOARCH ./cmd/$$binary; \
		done \
	done

# Install the binary
.PHONY: install
install: build
	@echo "Installing ${BINARY_NAME}..."
	sudo ./install.sh

# Run tests (CI-friendly: no coverage, fast feedback)
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage (LOCAL DEVELOPMENT ONLY)
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	go test -bench=. ./...

# Lint the code
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
		go fmt ./...; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf ${BIN_DIR}
	rm -rf dist/
	rm -f coverage.out coverage.html

# Create directories
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

dist:
	mkdir -p dist

# Show version information
.PHONY: version
version:
	@echo "Version: ${VERSION}"
	@echo "Git Commit: ${GIT_COMMIT}"
	@echo "Build Date: ${BUILD_DATE}"
	@echo "Go Version: ${GO_VERSION}"

# Calculate next semantic version
.PHONY: next-version
next-version:
	@if [ -f scripts/version.sh ]; then \
		./scripts/version.sh; \
	else \
		echo "scripts/version.sh not found"; \
	fi

# Validate commit messages
.PHONY: validate-commits
validate-commits:
	@echo "Validating commit messages..."
	@COMMITS=$$(git log --oneline --pretty=format:"%s" HEAD~5..HEAD 2>/dev/null || echo ""); \
	if [ -z "$$COMMITS" ]; then \
		echo "No commits to validate"; \
		exit 0; \
	fi; \
	PATTERN="^(feat|fix|docs|style|refactor|perf|test|chore|ci|build)(\(.+\))?(!)?: .+"; \
	VALID=true; \
	echo "$$COMMITS" | while IFS= read -r commit; do \
		if echo "$$commit" | grep -qE "$$PATTERN"; then \
			echo "✅ $$commit"; \
		else \
			echo "❌ $$commit"; \
			VALID=false; \
		fi; \
	done

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build-install  - Generate install script from templates"
	@echo "  build          - Build the binary"
	@echo "  build-dev      - Build with race detection for development"
	@echo "  build-all      - Cross-compile for all platforms"
	@echo "  install        - Build and install the service"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  bench          - Run benchmarks"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  tidy           - Tidy dependencies"
	@echo "  clean          - Clean build artifacts"
	@echo "  version        - Show version information"
	@echo "  next-version   - Calculate next semantic version"
	@echo "  validate-commits - Validate conventional commit messages"
	@echo "  help           - Show this help message"

# Development workflow targets
.PHONY: dev
dev: clean fmt lint test build

# CI target: essential checks only (fast feedback)
.PHONY: ci
ci: clean fmt test lint build-install build

# Coverage for CI (separate, non-blocking)
.PHONY: ci-coverage
ci-coverage:
	@echo "Generating coverage for CI..."
	go test -v -coverprofile=coverage.out ./...

# Quick development build and run scheduler
.PHONY: run
run: build
	$(BIN_DIR)/provisioner

# Individual binary build targets
.PHONY: build-provisioner build-workspacectl build-templatectl build-jobctl
build-provisioner: $(BIN_DIR)
	@echo "Building provisioner..."
	CGO_ENABLED=0 go build ${BUILD_FLAGS} ${LDFLAGS} -o ${BIN_DIR}/provisioner ./cmd/provisioner

build-workspacectl: $(BIN_DIR)
	@echo "Building workspacectl..."
	CGO_ENABLED=0 go build ${BUILD_FLAGS} ${LDFLAGS} -o ${BIN_DIR}/workspacectl ./cmd/workspacectl

build-templatectl: $(BIN_DIR)
	@echo "Building templatectl..."
	CGO_ENABLED=0 go build ${BUILD_FLAGS} ${LDFLAGS} -o ${BIN_DIR}/templatectl ./cmd/templatectl

build-jobctl: $(BIN_DIR)
	@echo "Building jobctl..."
	CGO_ENABLED=0 go build ${BUILD_FLAGS} ${LDFLAGS} -o ${BIN_DIR}/jobctl ./cmd/jobctl