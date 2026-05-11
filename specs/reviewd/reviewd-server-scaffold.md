---
title: Review Service Binary Scaffold
description: New cmd/reviewd binary — standalone HTTP server for review data, independent of the CLI
status: proposed
author: Claude <noreply@anthropic.com>
---

# Feature: Review Service Binary Scaffold

## Goal

Create a new standalone server binary (`cmd/reviewd/`) that runs the review storage HTTP service described in the storage proposal. This binary is independent of the existing CLI (`cmd/draft/`) and is designed for deployment (container, cloud, etc.). It provides configuration via environment variables, graceful shutdown, health checks, and structured logging.

## Acceptance Criteria

- [ ] New `cmd/reviewd/main.go` entry point that starts an HTTP server
- [ ] Configuration via environment variables: `DATABASE_URL`, `PORT`, `LOG_LEVEL`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`
- [ ] Graceful shutdown on SIGINT/SIGTERM
- [ ] `GET /healthz` returns 200 with `{"status":"ok"}`
- [ ] `GET /readyz` returns 200 when DB is connected, 503 otherwise
- [ ] Structured JSON logging to stdout
- [ ] Makefile target `build-reviewd` producing `bin/reviewd`
- [ ] Dockerfile for containerized deployment
- [ ] `make dev-db` target to start Postgres via podman
- [ ] Server compiles and starts (connecting to Postgres) without errors

## Approach

Create `cmd/reviewd/main.go` as the entry point. Configuration is read from environment variables using a simple config struct (no external config library). The server package lives in `internal/reviewd/` and exposes a `Server` struct that wires together middleware, routes, and a database connection. The initial route set is just health/readiness checks — actual API routes are added in subsequent specs.

Use `database/sql` with `github.com/lib/pq` for Postgres connectivity. The server opens a connection pool on startup and passes it to the readiness check.

## Affected Modules

- `cmd/reviewd/main.go` (new) — entry point, config parsing, signal handling, server startup
- `internal/reviewd/server.go` (new) — HTTP server struct, route registration, health/readiness
- `internal/reviewd/config.go` (new) — environment-based configuration
- `internal/reviewd/logging.go` (new) — structured JSON logger
- `Makefile` — add `build-reviewd` and `dev-db` targets
- `Dockerfile.reviewd` (new) — multi-stage build for the server binary
- `go.mod` — add `github.com/lib/pq` dependency

## Test Strategy

- Unit test: config parsing with defaults and overrides
- Unit test: health and readiness endpoints return correct status codes
- Integration test: server starts, connects to Postgres (podman), responds to healthz
- `make build-reviewd` succeeds without errors

## Out of Scope

- API routes for threads/comments/reviews (spec: reviewd-api)
- Database schema and migrations (spec: reviewd-migrations)
- Authentication middleware (spec: reviewd-auth)
- SSE/real-time (spec: reviewd-sse)

## Notes

- The server binary is entirely separate from the CLI — no shared main, no shared server code
- It reuses the data model types from `internal/review/types.go` where applicable
- Environment variables follow 12-factor conventions for deployment flexibility
