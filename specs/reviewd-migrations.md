---
title: Postgres Migrations and Schema
description: Database schema for review data with embedded migration tooling and podman-based local Postgres
status: proposed
author: Claude <noreply@anthropic.com>
---

# Feature: Postgres Migrations and Schema

## Goal

Define the Postgres schema for storing review data (repos, threads, comments, participants, reviews) and implement embedded SQL migrations that run automatically on server startup. The schema maps the existing JSON data model to relational tables with proper indexes for the API query patterns.

## Acceptance Criteria

- [ ] Migration files in `internal/reviewd/migrations/` as numbered `.sql` files
- [ ] Embedded migrations via `//go:embed` — no external migration tool required
- [ ] Migration runner that tracks applied migrations in a `schema_migrations` table
- [ ] Tables: `repos`, `participants`, `reviews`, `review_reviewers`, `threads`, `comments`
- [ ] Proper indexes on foreign keys and common query paths (repo_id + document, thread status)
- [ ] `version` column on `threads` table for optimistic concurrency
- [ ] `make dev-db` starts a Postgres container via podman with correct credentials
- [ ] Migrations run successfully against a fresh database on server startup
- [ ] Migrations are idempotent (re-running is safe)

## Approach

Use a simple embedded migration runner (no external dependency like goose/migrate). Migration files are numbered sequentially (`001_initial.sql`, `002_...`) and embedded into the binary. On startup, the runner creates `schema_migrations` if missing, checks which migrations have been applied, and runs pending ones in order within a transaction.

The schema uses UUIDs as primary keys (matching existing thread/comment IDs), timestamps with timezone, and JSONB for the anchor field (structured but rarely queried by subfield).

## Affected Modules

- `internal/reviewd/migrations/001_initial.sql` (new) — full initial schema
- `internal/reviewd/migrate.go` (new) — migration runner with embed
- `cmd/reviewd/main.go` — call migration runner on startup
- `Makefile` — `dev-db` target using podman

## Schema Design

```sql
-- Repositories registered with the service
CREATE TABLE repos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_owner TEXT NOT NULL,
    github_repo  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(github_owner, github_repo)
);

-- Participants (users)
CREATE TABLE participants (
    id    TEXT PRIMARY KEY,  -- hash-based ID from email
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reviews
CREATE TABLE reviews (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id    UUID NOT NULL REFERENCES repos(id),
    title      TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',
    documents  JSONB NOT NULL DEFAULT '[]',
    source_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Review-reviewer assignments
CREATE TABLE review_reviewers (
    review_id       UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    participant_id  TEXT NOT NULL REFERENCES participants(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    approved_at     TIMESTAMPTZ,
    approval_source_ref TEXT,
    PRIMARY KEY (review_id, participant_id)
);

-- Threads
CREATE TABLE threads (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id    UUID NOT NULL REFERENCES repos(id),
    review_id  UUID REFERENCES reviews(id),
    document   TEXT NOT NULL,
    anchor     JSONB NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',
    version    INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_threads_repo_document ON threads(repo_id, document);
CREATE INDEX idx_threads_repo_status ON threads(repo_id, status);

-- Comments
CREATE TABLE comments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id  UUID NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    author     TEXT NOT NULL REFERENCES participants(id),
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_comments_thread ON comments(thread_id);
```

## Test Strategy

- Unit test: migration runner applies migrations in order, skips already-applied ones
- Unit test: re-running migrations on an up-to-date DB is a no-op
- Integration test: fresh Postgres (podman) + run migrations + verify tables exist with correct columns
- The `dev-db` make target is tested by running it and connecting

## Out of Scope

- Data access layer / CRUD queries (spec: reviewd-storage)
- Seed data or test fixtures
- Migration rollback (forward-only for simplicity)
- Production database provisioning

## Notes

- The `anchor` field uses JSONB because it's a structured object stored/retrieved as a whole
- `documents` on reviews uses JSONB array since it's a simple string list
- `version` column on threads enables optimistic concurrency (If-Match header in API)
- Migration runner logs each applied migration for observability
