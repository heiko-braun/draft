---
title: REST API and Sync Endpoints
description: Full REST API surface with CRUD routes, optimistic concurrency, and bulk sync/publish
status: proposed
author: Claude <noreply@anthropic.com>
---

# Feature: REST API and Sync Endpoints

## Goal

Implement the complete REST API surface from the storage proposal. This includes CRUD endpoints for threads, comments, and reviews, scoped by repository. It includes the bulk sync and publish endpoints for offline-capable clients, and uses optimistic concurrency (version-based) for thread mutations.

## Acceptance Criteria

- [ ] Route prefix: `/api/v1/repos/{owner}/{repo}/`
- [ ] Thread endpoints:
  - `GET    /threads` — list all threads (optional `?document=` filter, `?status=` filter)
  - `GET    /threads/{id}` — get single thread with comments
  - `PUT    /threads/{id}` — create or update thread (requires `If-Match` header for updates)
  - `DELETE /threads/{id}` — delete thread (requires write permission)
- [ ] Comment endpoints:
  - `POST   /threads/{id}/comments` — add comment to thread
- [ ] Review endpoints:
  - `GET    /reviews` — list reviews
  - `POST   /reviews` — create review
  - `GET    /reviews/{id}` — get review detail
  - `PATCH  /reviews/{id}` — update review status, add/remove reviewers
- [ ] Sync endpoints:
  - `POST   /sync` — bulk pull: accepts `?since=<timestamp>`, returns all threads/comments changed since
  - `POST   /publish` — bulk push: accepts array of thread mutations, applies with conflict detection
- [ ] Optimistic concurrency: PUT to a thread with stale `If-Match` version returns `409 Conflict` with current state
- [ ] Comment on resolved thread auto-reopens it
- [ ] All responses use JSON with consistent error format: `{"error": "message"}`
- [ ] Write operations require write-level GitHub permission (enforced via auth context)
- [ ] Read operations require read-level permission

## Approach

Create `internal/reviewd/routes.go` for route registration and `internal/reviewd/handlers.go` for handler implementations. Handlers extract path parameters, validate input, call the store, and write JSON responses. The router uses Go 1.22+ `http.ServeMux` with method-and-path patterns (e.g., `GET /api/v1/repos/{owner}/{repo}/threads`).

The sync endpoint returns threads with `updated_at > since` parameter. The publish endpoint accepts an array of mutations (create/update/delete), each processed in sequence with individual success/failure results returned.

## Affected Modules

- `internal/reviewd/routes.go` (new) — route registration on ServeMux
- `internal/reviewd/handlers.go` (new) — all HTTP handler implementations
- `internal/reviewd/request.go` (new) — request parsing helpers, path param extraction
- `internal/reviewd/response.go` (new) — JSON response helpers, error formatting
- `internal/reviewd/server.go` — wire routes into the server

## API Details

### Sync (Pull)

```
POST /api/v1/repos/{owner}/{repo}/sync
Body: {"since": "2024-01-01T00:00:00Z"}
Response: {
  "threads": [...],
  "reviews": [...],
  "server_time": "2024-01-15T10:00:00Z"
}
```

Client stores `server_time` and uses it as `since` on next sync.

### Publish (Push)

```
POST /api/v1/repos/{owner}/{repo}/publish
Body: {
  "mutations": [
    {"op": "upsert_thread", "thread": {...}, "expected_version": 3},
    {"op": "add_comment", "thread_id": "...", "comment": {...}},
    {"op": "delete_thread", "thread_id": "..."}
  ]
}
Response: {
  "results": [
    {"index": 0, "ok": true, "thread": {...}},
    {"index": 1, "ok": true, "comment": {...}},
    {"index": 2, "ok": false, "error": "thread not found"}
  ]
}
```

### Conflict Response (409)

```
PUT /api/v1/repos/{owner}/{repo}/threads/{id}
If-Match: 3
→ 409 Conflict
Body: {"error": "version conflict", "current_version": 5, "thread": {...}}
```

## Test Strategy

- Integration tests for each endpoint against real Postgres
- Test CRUD lifecycle: create thread → add comment → resolve → verify state
- Test optimistic concurrency: update with correct version succeeds, stale version returns 409
- Test auto-reopen: comment on resolved thread changes status to open
- Test sync: create threads, sync with `since` in the past, verify all returned; sync with `since` in the future, verify empty
- Test publish: batch of mutations with one conflict, verify partial success
- Test permission enforcement: read-only user cannot create threads (403)

## Out of Scope

- Real-time push to clients (spec: reviewd-sse)
- SaaS session management and content proxy (future spec)
- CLI client integration
- Rate limiting

## Notes

- The API is versioned (`/api/v1/`) for forward compatibility
- Thread mutations increment version automatically — clients never set the version
- The publish endpoint processes mutations sequentially (not in parallel) to preserve ordering
- Error responses always include a machine-readable `error` field
