# Review SaaS: Design Proposal

## Problem

Today, `draft review` requires the repository to be cloned locally. Every reviewer must have the repo, run the CLI, and keep it running to participate. This creates friction:

- Reviewers who don't normally work in the repo must clone it just to leave comments.
- Non-technical stakeholders (PMs, tech writers, legal) can't participate at all.
- Reviews are ephemeral — they only exist while someone's laptop is serving the UI.
- No shared URL to point someone at ("go review section 3 of the auth spec").

## Concept

Move the review UI to a hosted service. The CLI becomes a thin client that registers context (repo, branch, user) with the service and receives a shareable URL. The service hosts the document viewer, renders annotations, and stores review data — no local server required.

**The local CLI remains the authoring entry point** — it knows the repo state, detects documents, and pushes content to the service. But the reading/reviewing experience lives in the browser at a stable URL.

## Architecture

```
┌─────────────────┐         ┌──────────────────────────┐
│  draft CLI      │         │  Review Service (SaaS)   │
│                 │ register│                          │
│ - detect repo   │────────►│  /api/v1/sessions/       │
│ - create session│         │    - repo + branch + ref │
│ - get URL       │◄────────│    - threads (comments)  │
│ - open browser  │   URL   │    - participants        │
└─────────────────┘         │                          │
                            │  Content: GitHub API     │
┌─────────────────┐         │    - fetch file tree     │
│  Browser        │────────►│    - render markdown     │
│  (any reviewer) │         │    - resolve anchors     │
└─────────────────┘         │                          │
                            │  UI: review.draft.dev    │
┌─────────────────┐         │    - document viewer     │
│  GitHub         │◄────────│    - inline annotations  │
│  (content API)  │  fetch  │    - real-time updates   │
└─────────────────┘         └──────────────────────────┘
```

The service never stores document content. It fetches directly from GitHub (via Contents API or git tree API) on demand, using the reviewer's OAuth token for access control. Content is cached transiently for rendering, not persisted.

## Workflow

### Author (has the repo)

```bash
$ draft review --remote
Registering review session...
Review URL: https://review.draft.dev/r/org/my-project/main

Share this URL with reviewers.
```

What happens:
1. CLI detects repo context (org, repo name, branch, current commit SHA).
2. CLI registers a review session with the service (repo coordinates + ref).
3. Service returns a stable URL.
4. CLI opens the URL in the browser (or prints it).

The author continues to work normally — push to GitHub as usual. The service always reads the latest content from the configured branch via GitHub API.

### Reviewer (no repo needed)

1. Opens URL in browser.
2. Authenticates via GitHub OAuth (service verifies repo read access via the user's token).
3. Service fetches document content from GitHub using the reviewer's token.
4. Reviewer reads documents, highlights text, adds comments.
5. Comments are stored server-side, visible to all participants immediately.

### Author (seeing remote comments locally)

```bash
$ draft review
# Local UI connects to the same backend — threads sync automatically
```

Or the author just uses the same shared URL in the browser.

## What the Service Hosts

| Component | Local (today) | SaaS |
|---|---|---|
| Document content | Read from filesystem | Fetched from GitHub API on demand |
| Document rendering/UI | Local HTTP server | Hosted at stable URL |
| Thread/comment storage | JSON in git worktree | Server-side DB |
| Inline highlighting | Local JS | Same JS, served from CDN |
| Auth | None (localhost) | GitHub OAuth |
| Real-time updates | N/A | SSE/WebSocket |
| Content storage | Filesystem | None — GitHub is the source of truth |

## What the CLI Does

The CLI becomes a lightweight companion to the backend, not a standalone review tool:

- **`draft review`** — checks for pending reviews and updates. Shows a summary: "3 open threads, 2 new comments since yesterday." Links to the backend URL for each active review.
- **`draft review --create`** — registers a new review session (repo + branch + document paths) with the backend. Returns a shareable URL.
- **`draft review --notify`** — signals reviewers that something is ready for review (triggers notifications via the backend).
- **Detects context:** repo identity, branch, current commit, user identity. Passes this to the backend so URLs resolve correctly.

The CLI does **not** serve a local UI, render documents, or host a review experience. The backend is the single place where reviews happen. The CLI is the entry point from the terminal — it creates review sessions and checks for activity, then hands off to the browser.

```bash
$ draft review
Open reviews for org/my-project (main):

  docs/auth-spec.md — 2 open threads, 1 new comment
    → https://review.draft.dev/r/org/my-project/main#docs/auth-spec.md

  specs/api-v2.md — 1 open thread
    → https://review.draft.dev/r/org/my-project/main#specs/api-v2.md

$ draft review --create
Review session created.
URL: https://review.draft.dev/r/org/my-project/feat/new-auth

Share this with reviewers.
```

## Auth & Access Control

- Service authenticates users via GitHub OAuth.
- Access check: call GitHub API to verify user has read access to the repo.
- Write access (commenting): requires at least triage permission on the repo.
- Admin actions (delete threads, close reviews): requires write permission.
- Repo visibility is inherited — private repos mean only collaborators can access the review URL.

## Content Retrieval Model

The service fetches content from GitHub on demand, never stores it persistently:

```
Reviewer opens review.draft.dev/r/org/repo/main
  → Service calls GitHub Contents API with reviewer's OAuth token
  → GET /repos/org/repo/contents/docs/auth-spec.md?ref=main
  → Render markdown, index paragraphs, compute anchors
  → Cache transiently (minutes, not hours) for rendering performance
```

**Why the reviewer's token:** Access control is enforced by GitHub, not the service. If the reviewer can't read the repo, the content fetch fails. No need to build a separate permission model for content.

**Anchor stability:** Anchors reference `file_hash + start/end offsets + excerpt`. When the underlying file changes on GitHub, the service re-fetches and attempts to re-locate anchors by excerpt matching. Stale anchors are flagged in the UI as "content changed" — same behavior as the local CLI today.

**Content caching:** Short-lived in-memory or CDN cache keyed by `(repo, path, commit SHA)`. Immutable at a given SHA, so cache invalidation is trivial. Branch-level requests resolve to a SHA first, then cache against that.

## Incremental Adoption

This doesn't have to be all-or-nothing. A phased approach:

**Phase 1: Backend + hosted UI**
- Service stores threads, fetches content from GitHub, serves the review UI.
- CLI registers sessions and checks for updates.
- Benefit: reviewers need nothing installed — just a browser.

**Phase 2: Real-time collaboration**
- SSE/WebSocket for live updates.
- Presence indicators (who's viewing what).
- Benefit: Google Docs-like review experience.

**Phase 3: Integrations**
- GitHub webhooks for push notifications (new commits invalidate anchors, notify reviewers).
- Slack/email notifications for review requests and responses.
- GitHub App installation for org-level content access.

## Tradeoffs

| Pro | Con |
|-----|-----|
| Reviewers need zero local setup | Service to build and operate |
| Stable, shareable URLs | Document content leaves the repo (trust boundary) |
| Non-technical stakeholders can participate | Adds latency vs. local filesystem reads |
| Real-time collaboration possible | Requires internet for full experience |
| Works across forks, orgs, platforms | Auth complexity (GitHub OAuth, token refresh) |
| Review history persists independent of clones | Cost scales with usage (storage, bandwidth) |
| No content pushed — GitHub remains source of truth | Coupled to GitHub API (rate limits, availability) |
| Private repo access enforced by GitHub, not the service | Reviewer's OAuth token must be trusted by service |

## Open Questions

1. **GitHub API rate limits:** With many reviewers fetching content, rate limits could bite. Per-user tokens help (each reviewer uses their own quota), but popular documents may need aggressive caching or use the git tree API for bulk fetches.
2. **Non-GitHub hosts:** This model is GitHub-specific. GitLab/Bitbucket support would need equivalent content APIs or a different content fetch strategy.
3. **Offline/disconnected use:** Is there any offline story, or is the backend strictly required? Could the CLI show cached thread summaries when offline?
4. **Multi-branch:** If reviews are branch-scoped, does switching branches create a new review session or update the existing one?
5. **Pricing model:** Free for public repos, paid for private? Per-seat? Per-repo?
6. **Private repos and GitHub App:** Using reviewer tokens means the service sees their token. A GitHub App installation model would let the service access repo content with its own credentials, scoped to repos the org has authorized.
