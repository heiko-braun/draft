.PHONY: build build-reviewd install clean test fmt vet install-hooks sync-templates dev-db dev-db-stop run run-reviewd help

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

build-reviewd: ## Build the reviewd server binary
	go build -ldflags="-s -w" -o bin/reviewd ./cmd/reviewd

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

dev-db: ## Start Postgres in podman (port 5434)
	@podman run -d --name draft-postgres \
		-e POSTGRES_USER=draft \
		-e POSTGRES_PASSWORD=draft \
		-e POSTGRES_DB=draft_reviews \
		-p 5434:5432 \
		postgres:17-alpine \
	&& echo "Postgres running on localhost:5434 (draft/draft)"

dev-db-stop: ## Stop and remove the Postgres container
	@podman stop draft-postgres 2>/dev/null; podman rm draft-postgres 2>/dev/null; echo "Postgres stopped"

run: ## Run the draft CLI (pass args via ARGS=)
	go run $(LDFLAGS) ./cmd/draft $(ARGS)

run-reviewd: ## Run the reviewd server (requires dev-db)
	go run ./cmd/reviewd
