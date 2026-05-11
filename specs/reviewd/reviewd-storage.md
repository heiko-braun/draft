---
title: Postgres-Backed Storage Layer
description: CRUD operations for repos, threads, comments, reviews, and participants backed by Postgres
status: proposed
author: Claude <noreply@anthropic.com>
---

# Feature: Postgres-Backed Storage Layer

## Goal

Implement a Postgres-backed data access layer that provides CRUD operations for all review entities. This replaces the file-based `Store` from `internal/review/store.go` with SQL queries against the schema defined in the migrations spec. The storage layer is used by the HTTP handlers and includes optimistic concurrency support for threads.

## Acceptance Criteria

- [ ] `Store` struct in `internal/reviewd/store.go` wrapping `*sql.DB`
- [ ] Repo operations: `GetOrCreateRepo(owner, repo) → Repo`
- [ ] Thread CRUD: create, get, list by repo+document, list all by repo, update status, delete
- [ ] Comment CRUD: add comment to thread, list comments by thread
- [ ] Review CRUD: create, get, list by repo, update status, add/remove reviewer
- [ ] Participant CRUD: get-or-create by ID+name+email
- [ ] Optimistic concurrency: `UpdateThread` takes expected version, returns conflict error if mismatch
- [ ] `MergeThreads` function adapted for server-side use (comment dedup, status resolution)
- [ ] All operations use parameterized queries (no SQL injection)
- [ ] Proper error types for not-found and version-conflict cases

## Approach

The `Store` struct holds a `*sql.DB` connection pool. Each method runs a single SQL query or a short transaction. Thread updates use `UPDATE ... WHERE version = $expected RETURNING version` to implement optimistic concurrency — if zero rows are affected, it's a conflict.

The data model types from `internal/review/types.go` are reused directly. The store translates between Go structs and SQL rows, handling JSONB marshaling for anchors and document lists.

## Affected Modules

- `internal/reviewd/store.go` (new) — all CRUD operations
- `internal/reviewd/errors.go` (new) — `ErrNotFound`, `ErrVersionConflict` sentinel errors
- Uses types from `internal/review/types.go` (Thread, Comment, Review, Participant, Anchor)

## Test Strategy

- Integration tests against a real Postgres instance (podman via `make dev-db`)
- Test each CRUD operation: create → read → verify fields
- Test optimistic concurrency: two concurrent updates, one gets conflict error
- Test comment deduplication in MergeThreads
- Test not-found errors for missing entities
- Test get-or-create idempotency for repos and participants

## Out of Scope

- HTTP handlers (spec: reviewd-api)
- Authentication/authorization checks (spec: reviewd-auth)
- Bulk sync/publish endpoints (spec: reviewd-api)
- File-based store migration tooling

## Notes

- The store is intentionally not an interface — it's a concrete struct. If we need to mock it for handler tests, we can introduce an interface later.
- Anchor is stored as JSONB and marshaled/unmarshaled as `review.Anchor`
- Thread `version` starts at 1 and increments on every successful update
- Comments are append-only — no update or delete operations
