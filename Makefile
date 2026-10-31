.PHONY: help all run fmt lint lint-ci vet test check quality tidy build build-nocheck build-all release-all install clean coverage coverage-pkg-check cover vuln e2e

help: ## This help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

# Ignore parent-directory go.work files so this module builds standalone.
export GOWORK := off

APP_NAME       = s7commctl
APP_SRC        = main.go
ARCHS          = linux/amd64 linux/arm64 linux/arm/v7 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
TAG            := $(shell git describe --tags --exact-match 2>/dev/null || echo "")
# Strip leading v for display/artifacts; fall back to dev when not in a git repo
VERSION        := $(shell desc=$$(git describe --tags --always --dirty 2>/dev/null); if [ -z "$$desc" ]; then echo dev; else echo "$$desc" | sed 's/^v//'; fi)
LDFLAGS        := -ldflags "-s -w -X main.version=$(VERSION) -X main.tag=$(TAG) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"
RELEASE_DIR   := release

all: test build build-all ## Test and build the application

run: ## Run the application
	@echo "Running $(APP_NAME)"
	@go run $(LDFLAGS) $(APP_SRC)

fmt: ## Format Go code
	@echo "Formatting code"
	@gofmt -w .
	@gofumpt -l -w . 2>/dev/null || true

# Per-package floor (TEST-012): the lowest package, cmd/s7commctl, measures 97.8 %.
COVER_PKG_MIN := 95

coverage: ## Run tests with coverage (writes coverage.out)
	@echo "Running coverage"
	@go test -count=1 -race -coverprofile=coverage.out -covermode=atomic ./...

coverage-pkg-check: coverage ## Fail when a package is below COVER_PKG_MIN
	@awk -v m="$(COVER_PKG_MIN)" 'NR > 1 { n[$$1] = $$2; if ($$3 > 0) h[$$1] = 1 } \
	END { for (k in n) { p = k; sub(/\/[^\/]*$$/, "", p); s[p] += n[k]; if (k in h) c[p] += n[k] } \
	for (p in s) { v = 100 * c[p] / s[p]; if (v < m) { printf "%s: %.1f%% < %s%%\n", p, v, m; bad = 1 } } exit bad }' coverage.out

cover: coverage ## Open coverage report in browser
	@echo "Opening coverage report"
	@go tool cover -html=coverage.out

lint: ## Run staticcheck
	@echo "Running staticcheck"
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not found. Install with:"; \
		echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	fi

lint-ci: ## Run golangci-lint v2 (required for .golangci.yml version 2)
	@echo "Running golangci-lint"
	@go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3 run ./...

vet: ## Run go vet on project packages
	@echo "Running go vet"
	@go vet ./...

test: ## Run all tests on project packages
	@echo "Running unit tests (race detector)"
	@go clean -testcache
	@go test -count=1 -race ./...

check: fmt lint lint-ci vet vuln test coverage-pkg-check ## Run all checks (format, lint, vet, vulnerability scan, test, per-package coverage)

quality: check ## Alias for full quality gate

tidy: ## Tidy go modules
	@echo "Tidying go modules"
	@go mod tidy

build: check ## Build the application
	@echo "Building $(APP_NAME)"
	@mkdir -p bin
	@go build $(LDFLAGS) -o ./bin/$(APP_NAME) $(APP_SRC)

build-nocheck: vet ## Build without running test/lint-ci
	@echo "Building $(APP_NAME) (no test/lint-ci)"
	@mkdir -p bin
	@go build $(LDFLAGS) -o ./bin/$(APP_NAME) $(APP_SRC)

build-all: ## Build the application for all architectures
	@mkdir -p $(RELEASE_DIR)
	@for arch in $(ARCHS); do \
		os=$${arch%%/*}; \
		rest=$${arch#*/}; \
		cpu=$${rest%%/*}; \
		variant=$${rest#*/}; \
		if [ "$$cpu" = "arm" ] && [ "$$variant" = "v7" ]; then \
			echo "Building $(APP_NAME)-$(VERSION)-$$os-armv7..."; \
			GOOS=$$os GOARCH=$$cpu GOARM=7 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(APP_NAME)-$(VERSION)-$$os-armv7 $(APP_SRC); \
		elif [ "$$os" = "windows" ]; then \
			echo "Building $(APP_NAME)-$(VERSION)-$$os-$$cpu.exe..."; \
			GOOS=$$os GOARCH=$$cpu go build $(LDFLAGS) -o $(RELEASE_DIR)/$(APP_NAME)-$(VERSION)-$$os-$$cpu.exe $(APP_SRC); \
		else \
			echo "Building $(APP_NAME)-$(VERSION)-$$os-$$cpu..."; \
			GOOS=$$os GOARCH=$$cpu go build $(LDFLAGS) -o $(RELEASE_DIR)/$(APP_NAME)-$(VERSION)-$$os-$$cpu $(APP_SRC); \
		fi \
	done

release-all: build-all ## Package the build binaries into tar.gz archives for all architectures
	@mkdir -p $(RELEASE_DIR)
	@for arch in $(ARCHS); do \
		os=$${arch%%/*}; \
		rest=$${arch#*/}; \
		cpu=$${rest%%/*}; \
		variant=$${rest#*/}; \
		if [ "$$cpu" = "arm" ] && [ "$$variant" = "v7" ]; then \
			bin=$(APP_NAME)-$(VERSION)-$$os-armv7; \
		elif [ "$$os" = "windows" ]; then \
			bin=$(APP_NAME)-$(VERSION)-$$os-$$cpu.exe; \
		else \
			bin=$(APP_NAME)-$(VERSION)-$$os-$$cpu; \
		fi; \
		if [ "$$os" = "windows" ]; then \
			echo "Packaging $${bin%.exe}.zip..."; \
			(cd $(RELEASE_DIR) && zip -q "$${bin%.exe}.zip" "$$bin"); \
		else \
			echo "Packaging $$bin.tar.gz..."; \
			tar czf $(RELEASE_DIR)/$$bin.tar.gz -C $(RELEASE_DIR) $$bin; \
		fi; \
	done

vuln: ## Scan the module and its dependencies for known vulnerabilities (govulncheck)
	@govulncheck ./...

e2e: ## Build the binary and run the e2e suite against it
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/s7commctl $(APP_SRC)
	@S7COMMCTL_E2E_BIN=$(CURDIR)/bin/s7commctl S7COMMCTL_E2E_VERSION=$(VERSION) go test -count=1 -tags=e2e ./e2e/...

install: build ## Install the built binary to /usr/local/bin
	@echo "Installing $(APP_NAME) to /usr/local/bin"
	@sudo install -m 0755 ./bin/$(APP_NAME) /usr/local/bin/$(APP_NAME)

clean: ## Clean the build artifacts
	@echo "Cleaning build artifacts"
	@rm -f $(APP_NAME)
	@rm -rf $(RELEASE_DIR)
	@rm -rf bin
	@rm -f coverage.out coverage.html
