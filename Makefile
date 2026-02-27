# Binary & paths
BINARY_NAME     := deeplink
CMD_DIR         := ./cmd
BUILD_DIR       := build
DIST_DIR        := dist
INSTALL_PREFIX  ?= /usr/local/bin

# Version info baked into binary at link time
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go toolchain
GO       := go
GOBIN    := $(shell $(GO) env GOPATH)/bin

# Lint timeout
GOLANGCI_LINT_TIMEOUT ?= 5m

# Colors
GREEN  := \033[0;32m
YELLOW := \033[0;33m
BLUE   := \033[0;34m
RED    := \033[0;31m
NC     := \033[0m

.PHONY: all
all: build

## build: Compile the binary
.PHONY: build
build:
	@echo "$(BLUE)Building $(BINARY_NAME) $(VERSION)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

## build-debug: Compile with debug symbols (no optimisations)
.PHONY: build-debug
build-debug:
	@echo "$(BLUE)Building debug binary...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(CMD_DIR)
	@echo "$(GREEN)✓ Debug binary: $(BUILD_DIR)/$(BINARY_NAME)-debug$(NC)"

## release: Cross-compile for macOS (arm64/amd64), Linux, Windows
.PHONY: release
release: clean
	@echo "$(BLUE)Cross-compiling for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	GOOS=darwin  GOARCH=arm64  $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64   $(CMD_DIR)
	GOOS=darwin  GOARCH=amd64  $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64   $(CMD_DIR)
	GOOS=linux   GOARCH=amd64  $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64    $(CMD_DIR)
	GOOS=linux   GOARCH=arm64  $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64    $(CMD_DIR)
	GOOS=windows GOARCH=amd64  $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "$(GREEN)✓ Release binaries in $(DIST_DIR)/$(NC)"
	@ls -lh $(DIST_DIR)

## run: Build and run with --help
.PHONY: run
run: build
	@echo "$(BLUE)Running $(BINARY_NAME)...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) --help

## test: Run all unit tests
.PHONY: test
test:
	@echo "$(BLUE)Running tests...$(NC)"
	$(GO) test -v -race ./...
	@echo "$(GREEN)✓ All tests passed$(NC)"

## test-coverage: Run tests and open HTML coverage report
.PHONY: test-coverage
test-coverage:
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"
	@$(GO) tool cover -func=coverage.out | grep total

## test-short: Run tests skipping slow/integration tests
.PHONY: test-short
test-short:
	@echo "$(BLUE)Running short tests...$(NC)"
	$(GO) test -short ./...

## lint: Run golangci-lint (falls back to go vet if not installed)
.PHONY: lint
lint:
	@echo "$(BLUE)Linting...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=$(GOLANGCI_LINT_TIMEOUT) ./...; \
	else \
		echo "$(YELLOW)golangci-lint not found, falling back to go vet$(NC)"; \
		echo "$(YELLOW)Install with: make tools$(NC)"; \
		$(GO) vet ./...; \
	fi
	@echo "$(GREEN)✓ Lint passed$(NC)"

## format: Format code with gofmt + gofumpt
.PHONY: format
format:
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GO) fmt ./...
	@if command -v gofumpt >/dev/null 2>&1; then \
		gofumpt -w .; \
	else \
		echo "$(YELLOW)gofumpt not found — only gofmt applied. Install with: make tools$(NC)"; \
	fi
	@echo "$(GREEN)✓ Code formatted$(NC)"

## format-check: Check formatting without writing files (CI-safe)
.PHONY: format-check
format-check:
	@echo "$(BLUE)Checking formatting...$(NC)"
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "$(RED)✗ Unformatted files (run 'make format'):$(NC)"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@if command -v gofumpt >/dev/null 2>&1; then \
		unformatted_gofumpt="$$(gofumpt -l .)"; \
		if [ -n "$$unformatted_gofumpt" ]; then \
			echo "$(RED)✗ gofumpt issues detected (run 'make format'):$(NC)"; \
			echo "$$unformatted_gofumpt"; \
			exit 1; \
		fi; \
	fi
	@echo "$(GREEN)✓ Formatting OK$(NC)"

## vet: Run go vet
.PHONY: vet
vet:
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ vet passed$(NC)"

## security: Check for known vulnerabilities (requires gosec)
.PHONY: security
security:
	@echo "$(BLUE)Checking for vulnerabilities...$(NC)"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "$(YELLOW)gosec not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest$(NC)"; \
	fi

## deps: Download and tidy dependencies
.PHONY: deps
deps:
	@echo "$(BLUE)Installing dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies ready$(NC)"

## update-deps: Upgrade all dependencies to latest
.PHONY: update-deps
update-deps:
	@echo "$(BLUE)Updating dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

## tools: Install dev tools via mise (see .mise.toml)
.PHONY: tools
tools:
	@echo "$(BLUE)Installing dev tools via mise...$(NC)"
	@if ! command -v mise >/dev/null 2>&1; then \
		echo "$(RED)✗ mise not found.$(NC)"; \
		echo "$(YELLOW)Install: curl https://mise.run | sh$(NC)"; \
		echo "$(YELLOW)Docs:    https://mise.jdx.dev$(NC)"; \
		exit 1; \
	fi
	mise install
	@echo "$(GREEN)✓ Tools installed (versions pinned in .mise.toml)$(NC)"

## install: Build and install binary to INSTALL_PREFIX (default: /usr/local/bin)
.PHONY: install
install: build
	@echo "$(BLUE)Installing to $(INSTALL_PREFIX)/$(BINARY_NAME)...$(NC)"
	@install -d $(INSTALL_PREFIX)
	@install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PREFIX)/$(BINARY_NAME)
	@echo "$(GREEN)✓ Installed: $(INSTALL_PREFIX)/$(BINARY_NAME)$(NC)"

## uninstall: Remove installed binary
.PHONY: uninstall
uninstall:
	@echo "$(BLUE)Uninstalling $(BINARY_NAME)...$(NC)"
	@if [ -f "$(INSTALL_PREFIX)/$(BINARY_NAME)" ]; then \
		rm -f $(INSTALL_PREFIX)/$(BINARY_NAME); \
		echo "$(GREEN)✓ Removed $(INSTALL_PREFIX)/$(BINARY_NAME)$(NC)"; \
	else \
		echo "$(YELLOW)$(BINARY_NAME) not found at $(INSTALL_PREFIX)/$(BINARY_NAME)$(NC)"; \
	fi

## install-hooks: Install pre-commit git hook (format-check + lint + test)
.PHONY: install-hooks
install-hooks:
	@echo "$(BLUE)Installing git hooks...$(NC)"
	@mkdir -p .githooks
	@printf '#!/bin/sh\nset -e\nmake format-check\nmake vet\nmake test-short\n' > .githooks/pre-commit
	@chmod +x .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "$(GREEN)✓ pre-commit hook installed (.githooks/pre-commit)$(NC)"

## clean: Remove build artifacts and coverage files
.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)✓ Clean$(NC)"

## dev: Full dev cycle — format, vet, lint, test, build
.PHONY: dev
dev: format vet lint test build
	@echo "$(GREEN)✓ Dev cycle complete — ready to ship!$(NC)"

## ci: What CI runs — no writes, strict checks
.PHONY: ci
ci: deps format-check vet lint test
	@echo "$(GREEN)✓ CI checks passed$(NC)"

## help: Show this help
.PHONY: help
help:
	@echo ""
	@echo "$(GREEN)$(BINARY_NAME)$(NC) $(VERSION) — Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^## / { \
		split($$0, a, ": "); \
		printf "  $(BLUE)%-22s$(NC) %s\n", a[2], substr($$0, index($$0, a[2]) + length(a[2]) + 2) \
	}' $(MAKEFILE_LIST) | sort
	@echo ""
	@echo "Variables:"
	@echo "  $(BLUE)INSTALL_PREFIX$(NC)  Install path (default: /usr/local/bin)"
	@echo "  $(BLUE)VERSION$(NC)         $(VERSION)"
	@echo "  $(BLUE)COMMIT$(NC)          $(COMMIT)"
	@echo ""