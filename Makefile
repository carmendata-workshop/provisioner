# Build variables
BINARY_NAME=provisioner
BIN_DIR=./bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION ?= $(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS = -ldflags "\
	-X environment-scheduler/pkg/version.Version=${VERSION} \
	-X environment-scheduler/pkg/version.GitCommit=${GIT_COMMIT} \
	-X environment-scheduler/pkg/version.BuildDate=${BUILD_DATE} \
	-w -s"

# Go build flags
BUILD_FLAGS = -a -installsuffix cgo

# Platforms for cross-compilation
PLATFORMS = linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64

# Default target
.PHONY: all
all: clean build

# Build the binary
.PHONY: build
build: $(BIN_DIR)
	@echo "Building ${BINARY_NAME} ${VERSION}..."
	CGO_ENABLED=0 go build ${BUILD_FLAGS} ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME} .

# Build for development (with race detection)
.PHONY: build-dev
build-dev: $(BIN_DIR)
	@echo "Building ${BINARY_NAME} for development..."
	go build -race ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME} .

# Cross-compile for all platforms
.PHONY: build-all
build-all: clean dist
	@echo "Cross-compiling for all platforms..."
	@for platform in $(PLATFORMS); do \
		echo "Building for $$platform..."; \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build ${BUILD_FLAGS} ${LDFLAGS} \
			-o dist/${BINARY_NAME}-$$GOOS-$$GOARCH .; \
	done

# Install the binary
.PHONY: install
install: build
	@echo "Installing ${BINARY_NAME}..."
	sudo ./install.sh

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
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

.PHONY: ci
ci: clean fmt test-coverage build-all

# Quick development build and run
.PHONY: run
run: build
	$(BIN_DIR)/$(BINARY_NAME)