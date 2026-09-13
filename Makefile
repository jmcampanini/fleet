.DEFAULT_GOAL := help

BUILD_DIR := build
BINARY := $(BUILD_DIR)/fleet
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || printf 'unknown')
LDFLAGS := -ldflags "-X github.com/jmcampanini/fleet/cmd.Version=$(VERSION)"

.PHONY: help build install test fmt fmt-check tidy tidy-check lint version-check vuln check clean

help: ## Show available targets.
	@printf 'Usage: make <target>\n\nTargets:\n'
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST) | LC_ALL=C sort

build: ## Build the versioned binary.
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -buildvcs=false $(LDFLAGS) -o $(BINARY) .

install: ## Install the CLI.
	go install .

test: ## Run all tests uncached with the race detector.
	go test -count=1 -race ./...

fmt: ## Format Go source files.
	go tool golangci-lint fmt

fmt-check: ## Verify formatting without changing files.
	go tool golangci-lint fmt --diff

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Verify module files without changing them.
	go mod tidy -diff

lint: ## Run static analysis.
	go tool golangci-lint run

version-check: build ## Verify the injected build version.
	@case "$(VERSION)" in unknown|n/a|"") echo "degenerate version identity: '$(VERSION)'"; exit 1;; esac
	@out="$$($(BINARY) --version)" || exit $$?; \
	if [ "$$out" != "fleet version $(VERSION)" ]; then \
		echo "version mismatch: got '$$out', want 'fleet version $(VERSION)'"; exit 1; \
	fi

vuln: ## Check reachable code and dependencies for known vulnerabilities.
	go tool govulncheck ./...

check: fmt-check tidy-check lint test build version-check vuln ## Run the complete local verification contract.

clean: ## Remove local build artifacts and the test cache.
	rm -rf $(BUILD_DIR)
	go clean -testcache
