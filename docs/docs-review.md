# `draft review` — Architecture Specification

**Collaborative review for spec-driven development.**

Version: 0.3.0
Date: 2026-05-11

---

## 1. Problem

Draft already solves the "spec before code" problem: `/spec` creates structured specifications, `/implement` builds from them, `/verify-spec` checks the result, and `/refine` evolves them. The `draft view` command renders specs in a browser for walkthroughs.

What's missing is the collaboration step between authoring and implementing. When a spec is written, there is no structured way to review, discuss, and approve it within the Draft workflow. Teams fall back to PR comments (line-oriented, awkward for prose), Slack threads (ephemeral, disconnected from the document), or meetings (synchronous, unrecorded). Review feedback doesn't live alongside the spec it refers to.

`draft review` fills this gap. It is a new subcommand that layers collaborative review — comments, threaded discussions, approvals — on top of the markdown specs and documents that Draft already manages, without modifying those files or requiring a separate platform. It completes the workflow: `/spec` → `draft review` → `/implement`.

---

## 2. Design Principles

**Service-backed.** Review data is stored in a Postgres database managed by the `reviewd` service. The CLI is a thin client that reads documents from the local filesystem and delegates all review operations (threads, comments, sync) to the service via REST API.

**Local-first reading.** The command launches from within an existing clone — the same context as every other Draft command. It reads documents from the filesystem and renders them locally. Only review data (threads, comments, reviews) lives on the server.

**Non-invasive.** Draft never modifies the user's working tree, index, or checked-out branch. It never writes to document files. It observes documents; it owns only its own review data.

**Real-time.** Mutations propagate immediately to the server. Other connected clients receive updates via Server-Sent Events. There is no batch-and-publish step — feedback is visible as soon as it's submitted.

---

## 3. System Architecture

```
┌─────────────────┐         ┌──────────────────────────┐
│  draft CLI      │         │  reviewd (service)       │
│                 │  REST   │                          │
│ - detect repo   │────────►│  /api/v1/repos/{o}/{r}/  │
│ - index docs    │         │    - threads (CRUD)      │
│ - serve UI      │◄────────│    - comments            │
│ - open browser  │   JSON  │    - reviews             │
└─────────────────┘         │    - sync / publish      │
                            │    - events (SSE)        │
┌─────────────────┐         │                          │
│  Browser        │────────►│  Auth: GitHub OAuth      │
│  (reviewer)     │  local  │  Storage: Postgres       │
└─────────────────┘  :8787  └──────────────────────────┘
```

The CLI starts a local HTTP server that serves the review UI and document content. All thread/comment/review operations are delegated to `reviewd` via its REST API. Authentication uses GitHub OAuth tokens — the same identity model as repo access.

---

## 4. Launch Model

`draft review` is always launched from within an existing git repository — the same context where all other Draft commands run. The repository anchors the session.

```
cd ~/code/acme-platform
draft review
```

On launch, the command:

1. Detects the git root of the current directory (same as other Draft commands).
2. Reads the remote origin to extract the GitHub owner and repo name.
3. Obtains a GitHub token (from `GITHUB_TOKEN` env or `gh auth token`).
4. Creates a remote client pointing to the `reviewd` service.
5. Scans configured document paths for markdown files (defaulting to `docs/` and `specs/`). Non-existent paths are silently skipped.
6. Starts a local HTTP server and opens the review UI in the user's browser.

### CLI entry points

| Command | Behavior |
|---|---|
| `draft review` | Opens the review UI for documents on the current branch. |
| `draft review --branch feature/auth-v2` | Opens focused on a specific branch. |
| `draft review specs/authentication.md` | Opens directly to a specific document. |
| `draft review --status` | Prints a summary of open reviews and pending changes to stdout. |
| `draft review --port 9000` | Override the default port (8787). |
| `draft review --debug` | Enable request logging to stderr. |

### Relationship to existing Draft commands

`draft review` is a peer to `draft init` and `draft view`. It shares the same project detection logic and respects the same `specs/` directory conventions. A typical workflow:

```
draft init                              # Bootstrap the project
/spec authentication                    # Create the spec (via AI assistant)
draft review specs/authentication.md    # Open for team review
/implement authentication               # Build it after approval
/verify-spec authentication             # Verify the implementation
```

There is no repo picker, no URL input, no dashboard of multiple repositories. Each invocation is scoped to one repo — the one the user is standing in. To review a different repo, the user opens a new session from that repo's directory.

---

## 5. The `reviewd` Service

`reviewd` is a standalone HTTP server binary (`cmd/reviewd/`) that stores and serves all review data. It is independent of the CLI and designed for deployment (container, cloud, local dev).

### Configuration

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://draft:draft@localhost:5434/draft_reviews?sslmode=disable` | Postgres connection string |
| `PORT` | `5100` | HTTP listen port |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `--debug` | — | CLI flag, sets log level to debug |

### Database

Postgres with embedded SQL migrations that run on startup. Schema tracks:

- **repos** — registered repositories (owner + name)
- **participants** — users, keyed by hash of email
- **reviews** — review metadata, status, document list
- **threads** — discussion threads with anchors, version counter
- **comments** — ordered by creation time, append-only

Migrations are embedded via `//go:embed` — no external migration tool required.

### Authentication

- Client sends GitHub OAuth token in `Authorization: Bearer` header.
- Service calls GitHub API (`GET /user`) to verify identity (cached 5 minutes).
- Repo access checked via `GET /repos/{owner}/{repo}` — permission mapped to read/write/admin.
- No separate user accounts — identity derived from GitHub profile.
- Health endpoints (`/healthz`, `/readyz`) bypass auth.

### API Surface

All data endpoints are scoped under `/api/v1/repos/{owner}/{repo}/`:

```
GET    /threads                    → list threads (?document=, ?status=)
GET    /threads/{id}               → get thread with comments
PUT    /threads/{id}               → create or update thread (If-Match for updates)
DELETE /threads/{id}               → delete thread

POST   /threads/{id}/comments      → add comment (auto-reopens resolved threads)

GET    /reviews                    → list reviews
POST   /reviews                    → create review
GET    /reviews/{id}               → get review with reviewers
PATCH  /reviews/{id}               → update status, add/remove reviewers

POST   /sync                       → bulk pull (returns changes since timestamp)
POST   /publish                    → bulk push (array of mutations)

GET    /events                     → SSE stream for real-time updates
```

### Optimistic Concurrency

Threads carry a `version` counter incremented on every mutation. Updates require an `If-Match` header with the expected version:

```
Client → PUT /threads/{id}  (If-Match: 7, body: {status: "resolved"})
Server → 200 OK             (version: 8, new state)
         — or —
Server → 409 Conflict       (current_version: 9, thread: {...})
```

On conflict, the client can merge and retry.

### Real-time Updates (SSE)

Connected clients receive live events via `GET /api/v1/repos/{owner}/{repo}/events`:

- `thread.created`, `thread.updated`, `thread.resolved`, `thread.reopened`, `thread.deleted`
- `comment.created`

Heartbeat every 30 seconds. Clients that disconnect use the `/sync` endpoint with a timestamp to catch up.

---

## 6. Document Indexing

On launch, `draft review` scans for markdown files in configured paths from the local filesystem.

### Default document paths

```
docs/
specs/
```

These paths are included by default. Non-existent paths are silently skipped. Additional paths are configurable.

### Spec front-matter awareness

Draft specs include YAML front-matter with metadata like `title`, `description`, `status`, and `author`. The document indexer extracts this front-matter and uses it for:

- Displaying the spec title (from front-matter `title` rather than inferring from the first heading).
- Showing spec status (`proposed`, `approved`, `implemented`) in the document list.
- Filtering by author.

Non-spec markdown files (those without Draft front-matter) are supported but fall back to heading-based title extraction.

### Index structure

For each markdown file, `draft review` parses and stores:

- **Front-matter.** Extracted YAML metadata, if present.
- **Heading tree.** The hierarchy of headings (`#`, `##`, etc.) with their text and nesting level.
- **Paragraph boundaries.** Start/end positions with content hashes for change detection.

The index is ephemeral — rebuilt on each launch from the current document state. It is not persisted.

---

## 7. Anchor System

Comments are anchored to specific locations in specific versions of a document. Anchors must survive document edits — paragraphs rewritten, sections reordered, content added or removed.

### Anchor data model

```json
{
  "file_hash": "a1b2c3d4e5f6...",
  "start": 1234,
  "end": 1289,
  "excerpt": "tokens should expire after 24 hours unless"
}
```

| Field | Purpose |
|---|---|
| `file_hash` | SHA-256 of the file content at annotation time. Detects whether offsets are still valid. |
| `start` | Character offset where the selection begins in the rendered text content. |
| `end` | Character offset where the selection ends. |
| `excerpt` | The selected text. Used for display and as a fallback for re-locating the annotation. |

### Resolution strategy

When displaying a document, `draft review` resolves each thread's anchor:

1. **Hash matches.** The file hasn't changed since annotation — character offsets are valid. Use `start`/`end` directly to highlight the text.
2. **Hash differs.** The file has been edited. Fall back to searching for the `excerpt` string in the rendered text. If found, highlight at the new position.
3. **Not found.** The excerpt no longer exists in the document. The thread is shown in the sidebar but cannot be placed inline.

---

## 8. Core Data Model

### Review

A review is the top-level workflow object. It groups discussion around one or more documents.

```json
{
  "id": "uuid",
  "title": "Authentication Spec Review",
  "status": "open",
  "documents": ["specs/authentication.md"],
  "source_ref": "abc123f",
  "reviewers": [
    { "participant_id": "abc123", "status": "pending" },
    { "participant_id": "def456", "status": "approved" }
  ],
  "created_at": "2026-05-08T10:00:00Z",
  "updated_at": "2026-05-08T14:30:00Z"
}
```

| Status | Meaning |
|---|---|
| `open` | Accepting feedback. |
| `closed` | Review closed without completing. |
| `merged` | Review completed and approved. |

### Thread

A discussion anchored to a document location.

```json
{
  "id": "uuid",
  "document": "specs/authentication.md",
  "anchor": { "file_hash": "...", "start": 100, "end": 150, "excerpt": "..." },
  "review_id": "uuid",
  "status": "open",
  "version": 3,
  "comments": [
    {
      "id": "uuid",
      "author": "participant-hash",
      "body": "Should we consider shorter expiry for admin tokens?",
      "created_at": "2026-05-08T11:15:00Z"
    }
  ],
  "created_at": "2026-05-08T11:15:00Z",
  "updated_at": "2026-05-08T11:42:00Z"
}
```

Thread status: `open`, `resolved`, `wontfix`.

### Participant

```json
{
  "id": "hash-of-email",
  "name": "Bob Chen",
  "email": "bob@acme.com"
}
```

Derived from GitHub identity on first authenticated request. Auto-created by the service.

---

## 9. Conflict Resolution

Conflicts are resolved per-operation type:

| Operation | Concurrent with | Resolution |
|---|---|---|
| Add comment | Add comment | No conflict — append both, sort by timestamp |
| Add comment | Resolve | Accept comment, auto-reopen thread |
| Add comment | Delete thread | Reject comment, return "thread deleted" |
| Resolve | Reopen | Last-write-wins by version |
| Resolve | Resolve | Idempotent — no conflict |
| Delete | Any mutation | Delete wins, reject other mutation |

The optimistic concurrency protocol (version counter + If-Match header) handles concurrent thread mutations. Comments are append-only and never conflict.

---

## 10. User Experience

### UI runtime

`draft review` opens the review UI in the user's default browser. The Go binary starts a local HTTP server (default port 8787) and opens the browser to it. No desktop framework needed.

### App structure

**Sidebar — Document browser.** Lists all markdown files found in configured paths. Each entry shows:

- Document title (from front-matter or first heading).
- Open thread count (badge).
- Spec status from front-matter when available.

**Center — Reading view.** Rendered markdown with:

- Document path displayed as a header above the content.
- Inline highlights on annotated text.
- Text selection opens a comment modal for creating a new thread.
- Clicking a highlight opens the thread in the right panel.

**Right panel — Thread detail.** Shows the active thread's comment history with:

- The anchor excerpt.
- Comment thread with author, timestamp, and body.
- Reply input.
- Thread status controls (resolve, reopen, delete).

### Status bar

Persistent across all views:

| Indicator | States |
|---|---|
| **Repo** | Repository name and current branch. |
| **Pending changes** | Always "no" — mutations are immediate. |
| **Sync** | No-op — data is always current via SSE. |
| **Publish** | No-op — mutations are sent immediately. |

### Key workflows

#### Reviewer flow

1. Launch `draft review` in the repo.
2. App connects to `reviewd`, showing the document list.
3. Open a spec flagged for review.
4. Read, select text, leave comments. Each comment is sent immediately to the server.
5. Other reviewers see comments in real-time via SSE.

#### Author flow

1. Create a spec with `/spec authentication`.
2. Launch `draft review specs/authentication.md` — app opens directly to the new spec.
3. See reviewer comments anchored inline.
4. Respond to comments, resolve threads.
5. Refine the spec with `/refine authentication` based on feedback.
6. Once approved, proceed to `/implement authentication`.

---

## 11. Configuration

### CLI Configuration

| Variable | Default | Description |
|---|---|---|
| `REVIEWD_URL` | `http://localhost:5100` | URL of the reviewd service |
| `GITHUB_TOKEN` | (from `gh auth token`) | GitHub OAuth token for auth |

### Document paths

Default scan paths: `docs/` and `specs/`. Configured in `ReviewConfig`:

```go
ReviewConfig{
    DocumentPaths: []string{"docs/", "specs/"},
    FilePatterns:  []string{"*.md", "*.mdx"},
    DefaultBranch: "main",
}
```

---

## 12. Technology Choices

### CLI binary

Draft is written in Go. The `review` subcommand is part of the same binary — it ships with Draft, no separate install. The Go layer handles command-line parsing, repo detection, document indexing, and the local UI server.

### reviewd binary

A separate Go binary (`cmd/reviewd/`) designed for deployment. Uses:

- `database/sql` + `github.com/lib/pq` for Postgres
- `net/http` with Go 1.22+ routing patterns
- Embedded SQL migrations via `//go:embed`
- Structured JSON logging

### UI

The frontend is embedded as a single HTML file with inline CSS and vanilla JavaScript — no build step, no framework dependency. It communicates with the local Go server via a JSON REST API. Uses the Geist font for visual consistency.

### Markdown rendering

`goldmark` renders markdown to HTML on the Go side. The frontend receives pre-rendered HTML.

### Text annotation

Character-offset-based anchoring with excerpt fallback. Highlights are rendered by walking DOM text nodes and wrapping matched ranges in `<mark>` elements.

---

## 13. Deployment

### Local development

```bash
make dev-db          # Start Postgres via podman (port 5434)
make run-reviewd     # Start reviewd on :5100
make build           # Build draft CLI
draft review         # Launch review UI
```

### Production

The `reviewd` binary is a single static Go binary. Deploy options:

- **Container:** `Dockerfile.reviewd` provides a multi-stage build (alpine base, ~15MB image).
- **Managed:** Fly.io / Railway with managed Postgres.
- **Self-hosted:** Docker Compose with Postgres.

---

## 14. Future Considerations

These are explicitly out of scope for v1 but inform architectural decisions.

**SaaS hosted UI.** Move the review UI to a hosted service with shareable URLs. The CLI becomes a thin client that registers sessions. Reviewers need nothing installed — just a browser. See `docs/review-saas-proposal.md`.

**Spec status integration.** When a review reaches "approved" status, automatically update the spec's YAML front-matter `status` field.

**`/review` slash command.** A `/review` command for AI assistants to initiate reviews, summarize feedback, or check approval status.

**Integration with PR workflows.** Surfacing review status as commit status checks. Posting summary comments on PRs.

**Multi-repo dashboard.** A launcher that remembers recently used repos and shows aggregate review status.

**Notifications.** Email or Slack notifications for review activity.

**Non-GitHub hosts.** GitLab/Bitbucket support via pluggable auth providers.

---

## Appendix A: API Quick Reference

```bash
# Health check
curl http://localhost:5100/healthz

# List threads (requires auth)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:5100/api/v1/repos/myorg/myrepo/threads

# Create thread
curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:5100/api/v1/repos/myorg/myrepo/threads/$(uuidgen) \
  -d '{"document":"docs/spec.md","anchor":{"file_hash":"...","start":0,"end":10,"excerpt":"hello"}}'

# Add comment
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:5100/api/v1/repos/myorg/myrepo/threads/THREAD_ID/comments \
  -d '{"body":"looks good!"}'

# SSE stream
curl -N -H "Authorization: Bearer $TOKEN" \
  http://localhost:5100/api/v1/repos/myorg/myrepo/events
```
