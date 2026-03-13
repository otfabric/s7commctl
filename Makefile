.PHONY: help all run fmt lint lint-ci vet test check quality tidy build build-nocheck build-all release-all install clean coverage cover

help: ## This help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

APP_NAME       = s7commctl
APP_SRC        = main.go
ARCHS          = linux/amd64 linux/arm64 linux/arm/v7 darwin/amd64 darwin/arm64
VERSION       := $(shell head -1 version.txt)
COMMIT        := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS       := -ldflags "-X 'github.com/otfabric/s7commctl/cmd/s7commctl.Version=$(VERSION)' -X 'github.com/otfabric/s7commctl/cmd/s7commctl.Commit=$(COMMIT)' -X 'github.com/otfabric/s7commctl/cmd/s7commctl.BuildDate=$(BUILD_DATE)'"
RELEASE_DIR   := release

all: test build build-all ## Test and build the application

run: ## Run the application
	@echo "Running $(APP_NAME)"
	@go run $(APP_SRC)

fmt: ## Format Go code
	@echo "Formatting code"
	@gofmt -w .
	@gofumpt -l -w . 2>/dev/null || true

lint: ## Run staticcheck
	@echo "Running staticcheck"
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not found. Install with:"; \
		echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	fi

lint-ci: ## Run golangci-lint
	@echo "Running golangci-lint"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

vet: ## Run go vet on project packages
	@echo "Running go vet"
	@go vet ./...

test: ## Run all tests on project packages
	@echo "Running unit tests (race detector)"
	@go clean -testcache
	@go test -count=1 -race ./...

check: fmt lint lint-ci vet test ## Run all checks (format, lint, vet, test)

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

install: build ## Install the built binary to /usr/local/bin
	@echo "Installing $(APP_NAME) to /usr/local/bin"
	@sudo install -m 0755 ./bin/$(APP_NAME) /usr/local/bin/$(APP_NAME)

clean: ## Clean the build artifacts
	@echo "Cleaning build artifacts"
	@rm -f $(APP_NAME)
	@rm -rf $(RELEASE_DIR)
	@rm -rf bin
	@rm -f coverage.out coverage.html

coverage: ## Run tests with coverage
	@echo "Running tests with coverage"
	@go test -count=1 -race -coverprofile=coverage.out -covermode=atomic ./...

cover: coverage ## Open coverage report in browser
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
