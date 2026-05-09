.PHONY: build build-reviewd install clean test fmt vet install-hooks sync-templates dev-db dev-db-stop

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

sync-templates:
	@./scripts/sync-templates.sh

build: sync-templates
	go build $(LDFLAGS) -o bin/draft ./cmd/draft

install: sync-templates
	go install $(LDFLAGS) ./cmd/draft

clean:
	rm -rf bin/
	rm -rf cmd/draft/templates/.claude/
	rm -rf cmd/draft/templates/.cursor/
	rm -rf cmd/draft/templates/.principles/
	rm -rf cmd/draft/templates/specs/

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

install-hooks:
	./scripts/install-git-hooks.sh

build-reviewd:
	go build -ldflags="-s -w" -o bin/reviewd ./cmd/reviewd

dev-db:
	@podman run -d --name draft-postgres \
		-e POSTGRES_USER=draft \
		-e POSTGRES_PASSWORD=draft \
		-e POSTGRES_DB=draft_reviews \
		-p 5434:5432 \
		postgres:17-alpine \
	&& echo "Postgres running on localhost:5434 (draft/draft)"

dev-db-stop:
	@podman stop draft-postgres 2>/dev/null; podman rm draft-postgres 2>/dev/null; echo "Postgres stopped"

run:
	go run $(LDFLAGS) ./cmd/draft $(ARGS)

run-reviewd:
	go run ./cmd/reviewd
