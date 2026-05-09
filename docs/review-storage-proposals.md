# Review Storage: Design Proposal

Proposal for replacing the git orphan branch as the remote storage layer for review data. Preserves the existing local data model (JSON files per thread) and the `Store` interface.

> **Note:** A GitHub Gist backend was considered and rejected. Secret gists have no access control — anyone with the URL can read all data, and only the gist owner can write. This makes it unsuitable for private repos and team collaboration.

---

## Thin HTTP Service

### Concept

A lightweight HTTP API that stores review data as JSON documents, keyed by repository and thread ID. The service is the single source of truth; clients sync by pulling latest state and pushing local mutations. Auth via GitHub OAuth — the service verifies repo access before allowing reads/writes.

### API Surface

```
GET    /api/v1/repos/{repo-id}/threads                    → list threads
GET    /api/v1/repos/{repo-id}/threads/{thread-id}        → get thread
PUT    /api/v1/repos/{repo-id}/threads/{thread-id}        → create/update thread
DELETE /api/v1/repos/{repo-id}/threads/{thread-id}        → delete thread
POST   /api/v1/repos/{repo-id}/sync                       → bulk pull (returns all changed since timestamp)
POST   /api/v1/repos/{repo-id}/publish                    → bulk push (accepts array of mutations)
```

### Auth Model

- Client sends GitHub OAuth token in `Authorization` header.
- Service calls GitHub API to verify the user has at least read access to the repo.
- Write access requires repo write or triage permission.
- No separate user accounts — identity derived from GitHub profile.

### Storage

Server-side storage is pluggable (Postgres, SQLite, S3 + index). MVP uses SQLite per repo — one file, simple backup, zero ops for small scale.

### Sync Model

Optimistic write-through with offline fallback. No explicit sync/publish step — mutations propagate immediately when online, queue locally when offline.

- **Online:** Each mutation (add comment, resolve, etc.) is sent to the server immediately. The local store is updated optimistically for instant UI response.
- **Offline:** Mutations queue in the local store. On reconnect, queued operations are replayed against the server with conflict resolution.
- **Incoming changes:** Server pushes events via SSE (Server-Sent Events) to connected clients. The UI updates in real-time without polling.

### Conflict Resolution

Conflicts are resolved per-operation type using the simplest strategy that produces correct results.

**Comments (append-only — no conflicts possible):**

Comments have unique IDs and timestamps. Multiple people commenting on the same thread concurrently is not a conflict — the server appends all comments and returns them sorted by `created_at`. Duplicates are rejected by ID. This matches the existing `MergeThreads` deduplication logic.

**Status changes (last-write-wins by timestamp):**

If Alice resolves a thread and Bob reopens it concurrently, the later timestamp wins. A new comment on a resolved thread auto-reopens it (matches GitHub/GitLab convention — a reply implies the discussion isn't done).

**Thread deletion (hard boundary):**

If a thread is deleted, any pending mutations against it are rejected with a clear error. The client discards queued operations for that thread.

**Optimistic concurrency protocol:**

Each thread carries a `version` counter, incremented on every server-side mutation.

```
Client → PUT /threads/{id}  (If-Match: version-7, body: mutation)
Server → 200 OK             (version-8, new state)
         — or —
Server → 409 Conflict       (current state at version-9)
Client → merge locally using MergeThreads(local, server)
Client → retry PUT          (If-Match: version-9, body: merged state)
```

**Conflict matrix:**

| Operation | Concurrent with | Resolution |
|---|---|---|
| Add comment | Add comment | No conflict — append both, sort by timestamp |
| Add comment | Resolve | Accept comment, auto-reopen thread |
| Add comment | Delete thread | Reject comment, return "thread deleted" |
| Resolve | Reopen | Last-write-wins by timestamp |
| Resolve | Resolve | Idempotent — no conflict |
| Delete | Any mutation | Delete wins, reject other mutation |

**Why this works without CRDTs:**

Review threads are append-heavy with infrequent status transitions. The conflict surface is small — two people rarely resolve/reopen the same thread simultaneously. Optimistic concurrency with a version counter is sufficient and far simpler than CRDTs. The existing `MergeThreads` function handles the merge step unchanged.

### Tradeoffs

| Pro | Con |
|-----|-----|
| Full access control tied to GitHub repo perms | Requires running a service (even if small) |
| Works across forks, orgs, any git host | Adds a network dependency |
| Real conflict detection via versioning | Auth token management for CLI users |
| Scales to any project size | Hosting cost (minimal but nonzero) |
| Could support non-GitHub hosts later | More moving parts to maintain |

### Deployment Options

- **Managed:** Single Go binary on Fly.io / Railway with SQLite (Litestream for backup). ~$5/month.
- **Self-hosted:** Docker container, Postgres or SQLite.
- **Serverless:** Cloudflare Workers + D1 (SQLite at edge).

### Migration Path

Same thread JSON format — `draft review migrate --to api --endpoint https://reviews.example.com` uploads existing data.
