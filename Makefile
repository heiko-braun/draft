.PHONY: build install clean test fmt vet install-hooks

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

build:
	go build $(LDFLAGS) -o bin/claudespec ./cmd/claudespec

install:
	go install $(LDFLAGS) ./cmd/claudespec

clean:
	rm -rf bin/

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

install-hooks:
	./scripts/install-git-hooks.sh

run:
	go run $(LDFLAGS) ./cmd/claudespec $(ARGS)
