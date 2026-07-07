.PHONY: build install clean test fmt vet install-hooks sync-templates run help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

help: ## Show this help
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

sync-templates:
	@./scripts/sync-templates.sh

build: sync-templates ## Build the draft CLI binary
	go build $(LDFLAGS) -o bin/draft ./cmd/draft

install: sync-templates ## Install the draft CLI to $GOPATH/bin
	go install $(LDFLAGS) ./cmd/draft

clean: ## Remove build artifacts
	rm -rf bin/
	rm -rf cmd/draft/templates/.claude/
	rm -rf cmd/draft/templates/.cursor/
	rm -rf cmd/draft/templates/.principles/
	rm -rf cmd/draft/templates/specs/

test: ## Run all tests
	go test ./...

fmt: ## Format Go source files
	go fmt ./...

vet: ## Run go vet on all packages
	go vet ./...

install-hooks: ## Install git pre-commit hooks
	./scripts/install-git-hooks.sh

run: ## Run the draft CLI (pass args via ARGS=)
	go run $(LDFLAGS) ./cmd/draft $(ARGS)
