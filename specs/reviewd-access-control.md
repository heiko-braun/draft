---
title: Reviewd Access Control
description: Wire GitHub repo permission checks into all API routes to enforce read/write authorization
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Reviewd Access Control

## Goal

Enforce GitHub repo-level permissions on all reviewd API routes so that only users with appropriate access to the underlying GitHub repository can read or modify review data. Prevents unauthorized access to private repo reviews and ensures write operations require push/triage permission.

## Acceptance Criteria

- [ ] All read endpoints (GET threads, reviews, sync) require at least `read` (pull) permission on the GitHub repo
- [ ] All write endpoints (PUT/POST/DELETE threads, comments, reviews, publish) require at least `write` (push/triage) permission
- [ ] Requests from users without repo access return 403 with clear error message
- [ ] Repo access results are cached (already implemented, 5-minute TTL) to avoid excessive GitHub API calls
- [ ] Health/readiness endpoints remain unauthenticated

## Approach

Wrap each route handler in `server.go` with `s.auth.RequireRepoAccess(level, handler)`. The middleware already exists and is tested — it extracts `{owner}/{repo}` from the path, calls GitHub's repo API with the user's token, and maps permissions to `AccessRead`/`AccessWrite`/`AccessAdmin`. This is purely a wiring change in the `routes()` function.

## Affected Modules

- `internal/reviewd/server.go` — wrap route handlers with `RequireRepoAccess` at appropriate levels
- `internal/reviewd/handlers_test.go` — update tests to verify 403 behavior (mock GitHub API returns no access)

## Test Strategy

- Integration test: request with token that has no repo access → 403
- Integration test: request with read-only token → GET succeeds, PUT/POST/DELETE returns 403
- Integration test: request with write token → all operations succeed
- Existing tests continue to pass (they inject AuthContext directly, bypassing middleware)

## Out of Scope

- Per-thread ownership (e.g., "only the author can delete their thread")
- Fine-grained permissions beyond GitHub's repo-level model
- OAuth login flow or token exchange
- Rate limiting on GitHub API calls
