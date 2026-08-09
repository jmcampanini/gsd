.DEFAULT_GOAL := help

BUILD_DIR := build
BINARY := $(BUILD_DIR)/gsd
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || printf 'unknown')
LDFLAGS := -ldflags "-X github.com/jmcampanini/gsd/cmd.Version=$(VERSION)"

.PHONY: help build test lint vuln fmt fmt-check tidy tidy-check check clean

help: ## Show available targets.
	@printf 'Usage: make <target>\n\nTargets:\n'
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST) | LC_ALL=C sort

build: ## Build build/gsd with git-derived version metadata.
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BINARY) .

test: ## Run all tests uncached with the race detector.
	go test -count=1 -race ./...

lint: ## Run static analysis.
	go tool golangci-lint run

vuln: ## Check dependencies and reachable code for known vulnerabilities.
	go tool govulncheck ./...

fmt: ## Format Go source files.
	go tool golangci-lint fmt

fmt-check: ## Verify formatting without changing files.
	go tool golangci-lint fmt --diff

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Verify go.mod and go.sum are tidy without changing them.
	go mod tidy -diff

check: fmt-check tidy-check lint test build vuln ## Run the complete local verification contract.

clean: ## Remove build artifacts and the Go test cache.
	rm -rf $(BUILD_DIR)
	go clean -testcache
