# `draft review` — Architecture Specification

**Collaborative review for spec-driven development.**

Version: 0.2.0
Date: 2026-05-08

---

## 1. Problem

Draft already solves the "spec before code" problem: `/spec` creates structured specifications, `/implement` builds from them, `/verify-spec` checks the result, and `/refine` evolves them. The `draft view` command renders specs in a browser for walkthroughs.

What's missing is the collaboration step between authoring and implementing. When a spec is written, there is no structured way to review, discuss, and approve it within the Draft workflow. Teams fall back to PR comments (line-oriented, awkward for prose), Slack threads (ephemeral, disconnected from the document), or meetings (synchronous, unrecorded). Review feedback doesn't live alongside the spec it refers to.

`draft review` fills this gap. It is a new subcommand that layers collaborative review — comments, threaded discussions, approvals — on top of the markdown specs and documents that Draft already manages, without modifying those files or requiring a separate platform. It completes the workflow: `/spec` → `draft review` → `/implement`.

---

## 2. Design Principles

**Git-native.** Review data lives in the same repository as the documents it annotates, stored on a dedicated virtual branch. No external databases, no SaaS dependency. If you can push to the repo, you can participate in a review.

**Local-first.** The command launches from within an existing clone — the same context as every other Draft command. It reads documents from the filesystem, manages its own state via git worktrees, and works offline. Sync is explicit and user-initiated.

**Non-invasive.** Draft never modifies the user's working tree, index, or checked-out branch. It never writes to document files. It observes documents; it owns only its own review data.

**Batch-and-publish.** Review actions (comments, resolutions, approvals) accumulate locally as pending changes. The user publishes them as a deliberate act, like committing. This respects the reviewer's process of composing feedback across a document before making it visible.

---

## 3. Launch Model

`draft review` is always launched from within an existing git repository — the same context where all other Draft commands run. The repository anchors the session.

```
cd ~/code/acme-platform
draft review
```

On launch, the command:

1. Detects the git root of the current directory (same as other Draft commands).
2. Reads the remote origin to identify the repository.
3. Reads git config for user identity (`user.name`, `user.email`).
4. Locates or creates its worktrees (see §5).
5. Scans configured document paths for markdown files (defaulting to `docs/` and `specs/`). Non-existent paths are silently skipped.
6. Opens the review UI in the user's browser, consistent with how `draft view` works.

### CLI entry points

| Command | Behavior |
|---|---|
| `draft review` | Opens the review UI for documents on the current branch. |
| `draft review --branch feature/auth-v2` | Opens focused on a specific branch. |
| `draft review specs/authentication.md` | Opens directly to a specific document. |
| `draft review --sync` | Fetches latest review data without opening the UI. |
| `draft review --status` | Prints a summary of open reviews and pending changes to stdout. |
| `draft review --port 9000` | Override the default port (8787). |
| `draft review --debug` | Enable request logging to stderr. |

The `--sync` and `--status` flags support headless/CI workflows without launching the UI. They complement the interactive review experience with scriptable access to review state.

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

## 4. Virtual Review Branch

All review data — threads, comments, approvals, review metadata — is stored on a dedicated git branch that never merges into any code branch.

**Branch name:** `draft/reviews`

This branch contains only review data. It has no relationship to the repository's main branch history; it is an orphan branch rooted from an empty commit. It is pushed to and fetched from the same remote as the code, inheriting the repository's access control and transport credentials.

### Why a virtual branch

- Keeps review data out of the working tree and commit history of code branches.
- Uses git as the storage, sync, and transport layer with zero additional infrastructure.
- Inherits authentication and permissions from the repository.
- Provides full history and auditability of all review actions via the branch's commit log.

### Data layout

```
draft/reviews
├── schema-version              # Data format version for migration support
├── config.json                 # Document paths, branch defaults, settings
├── participants/
│   └── {user-hash}.json        # Display name, email, preferences
├── reviews/
│   └── {review-id}.json        # Review metadata, status, participants
└── threads/
    └── {document-path}/
        └── {thread-id}.json    # Thread with anchor, comments, status
```

Threads are nested under the document path they annotate. This makes per-document lookups a directory read and keeps the file count per directory manageable.

---

## 5. Worktree Strategy

`draft review` creates two git worktrees from the user's existing clone. Both are stored outside the repository working tree, under Draft's data directory.

### Document worktree

**Location:** `~/.draft/worktrees/{repo-id}/docs`

A sparse checkout of the source branch, containing only the configured document paths. Used to read document content for rendering. Updated on sync to reflect the latest remote state.

When documents are modified locally (the user is authoring), `draft review` reads from the user's actual working tree instead. The document worktree serves as the fallback for reviewers who only need the committed version.

### Review worktree

**Location:** `~/.draft/worktrees/{repo-id}/reviews`

A full checkout of the `draft/reviews` branch. `draft review` reads and writes review data as ordinary files here, then commits and pushes. This avoids the need for git plumbing commands for routine operations.

### Worktree lifecycle

| Event | Action |
|---|---|
| First launch in a repo | Create both worktrees. Initialize `draft/reviews` as an orphan branch if it doesn't exist. |
| Subsequent launch | Verify worktrees exist and are valid. Re-create if broken (e.g., parent clone was moved). |
| Sync (documents) | Fetch + checkout latest remote ref in the document worktree. |
| Sync (reviews) | Fetch + fast-forward the review worktree. |
| User deletes their clone | Worktrees break. Next launch detects this and prompts to re-initialize. |

### Reading from the user's working tree

For authors actively editing documents, `draft review` offers a "live" mode: read document content from the actual working tree (the user's checkout) rather than the document worktree. This shows uncommitted changes in the review context.

Live mode is the default when local modifications exist in watched document paths. The review data is always managed independently via the review worktree.

---

## 6. Document Indexing

On launch and on sync, `draft review` scans for markdown files in configured paths.

### Default document paths

```
docs/
specs/
```

These paths are included by default. Non-existent paths are silently skipped, so repos without a `docs/` or `specs/` directory work without configuration. Additional paths are configurable via `config.json` on the review branch.

### Spec front-matter awareness

Draft specs include YAML front-matter with metadata like `title`, `description`, `status`, and `author`. The document indexer extracts this front-matter and uses it for:

- Displaying the spec title (from front-matter `title` rather than inferring from the first heading).
- Showing spec status (`proposed`, `approved`, `implemented`) in the document list.
- Filtering by author.

Non-spec markdown files (those without Draft front-matter) are supported but fall back to heading-based title extraction.

### Index structure

For each markdown file, `draft review` parses and stores:

- **Front-matter.** Extracted YAML metadata, if present.
- **Heading tree.** The hierarchy of headings (`#`, `##`, etc.) with their text and nesting level. This powers structural navigation and anchor resolution.
- **Paragraph boundaries.** Start/end positions mapped to the heading tree, so each paragraph can be addressed as "the 3rd paragraph under Heading X > Subheading Y."
- **Content hashes.** Per-paragraph content hashes for change detection and anchor matching.
- **Document metadata.** Word count, last-modified timestamp from git.

The index is ephemeral — rebuilt on each launch/sync from the current document state. It is not persisted to the review branch.

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

This approach is simple and works for any content type (paragraphs, headings, table cells, list items) without requiring structural parsing of the HTML.

---

## 8. Core Data Model

### Review

A review is the top-level workflow object. It groups discussion around one or more documents at a specific point in time.

```json
{
  "id": "r_20260508_authentication",
  "title": "Authentication Spec Review",
  "author": "alice@acme.com",
  "status": "open",
  "created_at": "2026-05-08T10:00:00Z",
  "updated_at": "2026-05-08T14:30:00Z",
  "source_branch": "main",
  "source_ref": "abc123f",
  "documents": [
    "specs/authentication.md"
  ],
  "reviewers": [
    { "user": "bob@acme.com", "status": "pending" },
    { "user": "carol@acme.com", "status": "approved", "ref": "abc123f" }
  ]
}
```

| Status | Meaning |
|---|---|
| `open` | Accepting feedback. |
| `in_review` | All reviewers have been invited, discussion is active. |
| `resolved` | Author considers the review complete. |
| `archived` | Historical record, no longer active. |

### Thread

A discussion anchored to a document location.

```json
{
  "id": "t_a1b2c3d4",
  "review_id": "r_20260508_authentication",
  "document": "specs/authentication.md",
  "anchor": { },
  "status": "open",
  "created_at": "2026-05-08T11:15:00Z",
  "comments": [
    {
      "id": "c_001",
      "author": "bob@acme.com",
      "timestamp": "2026-05-08T11:15:00Z",
      "body": "Should we consider shorter expiry for admin tokens?"
    },
    {
      "id": "c_002",
      "author": "alice@acme.com",
      "timestamp": "2026-05-08T11:42:00Z",
      "body": "Good point. I'll add a tier-based expiry table."
    }
  ]
}
```

Thread status: `open`, `resolved`, `wont_fix`.

### Approval

Approvals are tracked per-reviewer within the review object. Each approval records the `source_ref` it applies to. If the document changes (new commit on the source branch), existing approvals are marked **stale** — the approval stands but is flagged as applying to a previous version.

### Participant

```json
{
  "email": "bob@acme.com",
  "display_name": "Bob Chen",
  "avatar_hash": "e3b0c44..."
}
```

Derived from git config on first use. Stored on the review branch so display names are consistent across participants.

---

## 9. Sync and Conflict Resolution

### Sync operations

| Operation | What happens |
|---|---|
| **Fetch documents** | `git fetch origin main` in the document worktree, then checkout the latest ref. Triggers document re-indexing and anchor resolution for all open threads. |
| **Fetch reviews** | `git fetch origin draft/reviews` in the review worktree, then fast-forward. |
| **Publish** | Commit all pending changes in the review worktree, then `git push origin draft/reviews`. |

### Conflict handling

Conflicts occur when two users modify review data concurrently. Because each thread is a separate file, conflicts only arise when two people comment on the same thread at the same time.

**Resolution strategy:**

1. User clicks "Publish."
2. App attempts `git push`. If rejected (non-fast-forward):
3. App runs `git fetch` + `git rebase` on the review branch.
4. If the rebase is clean (no file-level conflicts), push succeeds automatically.
5. If there's a file conflict (same thread file modified), the app performs a semantic merge: parse both versions of the thread JSON, combine comment arrays (deduplicate by comment ID, order by timestamp), take the most recent status. Write the merged result, commit, push.
6. If semantic merge fails for any reason, the app keeps both versions and flags the thread for manual resolution.

This makes conflicts nearly invisible in practice. Two people commenting on different threads: no conflict. Two people commenting on the same thread: auto-merged. Two people resolving the same thread differently: flagged.

---

## 10. User Experience

### UI runtime

`draft review` opens the review UI in the user's default browser, consistent with `draft view`. The Go binary starts a local HTTP server and opens the browser to it. No desktop framework (Tauri, Electron) is needed — this reuses a pattern Draft users already know.

### App structure

The UI has three primary zones:

**Sidebar — Document browser.** Lists all markdown files found in the configured document paths on the current branch. Each entry shows:

- Document title (from front-matter or first heading).
- Spec status from front-matter (`proposed`, `approved`, etc.) when available.
- Open thread count (badge).
- Review status indicator (not reviewed / in progress / approved / changes requested).
- Whether the user has unread activity on this document.

Supports filtering: "with open threads," "needs my review," "recently changed," "all."

**Center — Reading view.** Rendered markdown with inline highlights on annotated text. The user reads the document here.

- Selecting any text opens a comment modal for creating a new thread.
- Existing annotations are highlighted inline (blue underline for open, faded green for resolved). Clicking a highlight opens the thread in the right panel.
- Highlights use character offsets when the file is unchanged, falling back to excerpt search when the file has been edited.

**Right panel — Thread detail.** Shows the active thread's comment history, with a reply input at the bottom. Also shows:

- Thread status controls (resolve, reopen, delete).
- The anchor context — the original excerpt the thread was placed on.

### Status bar

Persistent across all views, showing:

| Indicator | States |
|---|---|
| **Repo** | Repository name and current branch. |
| **Pending changes** | Shown when unpublished local changes exist. |
| **Sync** | Button to fetch latest review data. Shows "Syncing..." during operation with toast confirmation on completion. |
| **Publish** | Button to commit and push pending changes. Shows "Publishing..." during operation with toast confirmation or error. |

The pending changes indicator is the most important. It must be visible at all times so the user always knows whether they have work that hasn't been shared. Both Sync and Publish show progress states and toast notifications for success/failure.

### Key workflows

#### Reviewer flow

1. Launch `draft review` in the repo.
2. App syncs automatically, showing the document list.
3. Open a spec flagged for review.
4. Read, select text, leave comments. Each comment creates a local pending change.
5. Open another document, leave more comments.
6. Review the pending changes summary.
7. Click "Publish" — all comments are committed and pushed to the review branch.

#### Author flow

1. Create a spec with `/spec authentication` in their AI assistant.
2. Launch `draft review specs/authentication.md` — app opens directly to the new spec.
3. See reviewer comments anchored to the current document state.
4. Respond to comments, resolve threads where changes have been made.
5. Publish responses.
6. Refine the spec with `/refine authentication` if needed based on feedback.
7. Once approved, proceed to `/implement authentication`.

#### Branch-based review (pre-PR)

1. Author creates a spec on `feature/new-api-spec` branch.
2. Runs `draft review --branch feature/new-api-spec`.
3. Creates a review, invites reviewers.
4. Reviewers run the same command, see the documents and begin commenting.
5. Discussion happens entirely within Draft, before any PR is opened.
6. When the spec is stable, the author opens a PR. The review history persists on `draft/reviews` for reference.

---

## 11. Configuration

Stored in `config.json` on the `draft/reviews` branch. Applies to all participants.

```json
{
  "document_paths": [
    "docs/",
    "specs/"
  ],
  "file_patterns": ["*.md", "*.mdx"],
  "default_branch": "main"
}
```

The default paths are `docs/` and `specs/`. Non-existent paths are silently skipped. Additional paths can be added for teams that keep documents in other locations.

User-local preferences (UI state, filters, notification settings) are stored in `~/.draft/review-config.json`, not on the review branch.

---

## 12. Technology Choices

### Relationship to the Draft CLI

Draft is written in Go. The `review` subcommand is part of the same binary — it ships with Draft, no separate install. The Go layer handles command-line parsing, git operations, worktree management, and launching the review UI.

### UI runtime

The review UI opens in the user's browser, consistent with how `draft view` already works. The Go binary starts a local HTTP server and opens the browser to it. This avoids the complexity of bundling a desktop framework while reusing a pattern Draft users already know.

The frontend is embedded as a single HTML file with inline CSS and vanilla JavaScript — no build step, no framework dependency. It communicates with the local Go server via a JSON REST API. The UI uses the same Geist font and markdown styling as `draft present` for visual consistency.

### Key libraries and tools

**Markdown rendering:** `goldmark` on the Go side (shared with `draft present`) renders markdown to HTML. The frontend receives pre-rendered HTML.

**Text annotation:** Character-offset-based anchoring with excerpt fallback. Highlights are rendered by walking DOM text nodes and wrapping matched ranges in `<mark>` elements. No external annotation library is required.

**Git operations:** Shell out to `git` for worktree operations, fetch, and push. The user's git is already configured with credentials, SSH agents, and any custom helpers. Using their git binary means Draft inherits all of that without reimplementing credential management. This is consistent with how Draft already operates.

**Review branch initialization:** The orphan branch is created in a temporary directory (`git init` + push) rather than using `git checkout --orphan` in the user's working tree. This ensures the user's working tree, index, and untracked files are never disturbed. Commits on the review branch use `--no-verify` since they contain only JSON data files.

---

## 13. Future Considerations

These are explicitly out of scope for v1 but inform architectural decisions.

**Spec status integration.** When a review reaches "approved" status, automatically update the spec's YAML front-matter `status` field from `proposed` to `approved`. This bridges the review workflow back into Draft's spec lifecycle — an approved spec is ready for `/implement`.

**`/review` slash command.** In addition to the `draft review` CLI subcommand, a `/review` slash command for AI assistants could initiate a review request, summarize open feedback, or check approval status — extending Draft's existing AI-assistant integration pattern (`/spec`, `/implement`, `/verify-spec`, `/refine`).

**Integration with PR workflows.** Surfacing review status as a commit status check on PRs. Posting a summary comment on PRs that link to the full review. This requires a lightweight server component or GitHub Action.

**Remote sync service.** For teams that want real-time collaboration without git push/fetch latency, a relay server that syncs review data via websockets. The data model is designed to support this — the JSON file structure maps directly to API resources.

**Multi-repo dashboard.** A launcher that remembers recently used repos and shows aggregate review status. The single-repo-per-session model is preserved; the dashboard is just a convenience for navigation.

**Notifications.** Email or Slack notifications when a review is requested, when new comments appear, when a document is approved. Requires a server-side component watching the review branch.

**Document diffing.** Prose-aware structural diffs between document versions — "this paragraph was rewritten," "a new section was added" — rather than line-level diffs. The heading tree and content hashes in the index provide the foundation for this.

---

## Appendix A: Git Operations Reference

### Initialize review branch (first launch)

```bash
# Create orphan branch safely in a temp directory (never touches working tree)
tmpdir=$(mktemp -d)
cd "$tmpdir"
git init
mkdir -p threads reviews participants
echo '1' > schema-version
echo '{"document_paths":["docs/","specs/"],"file_patterns":["*.md","*.mdx"],"default_branch":"main"}' > config.json
touch threads/.gitkeep reviews/.gitkeep participants/.gitkeep
git add .
git commit --no-verify -m "Initialize draft review data"
git remote add origin <remote-url>
git push origin HEAD:refs/heads/draft/reviews

# Back in the user's repo, fetch the new branch
cd /path/to/repo
git fetch origin draft/reviews:draft/reviews
rm -rf "$tmpdir"
```

### Create worktrees

```bash
# Document worktree (sparse)
git worktree add ~/.draft/worktrees/{repo-id}/docs origin/main --no-checkout
cd ~/.draft/worktrees/{repo-id}/docs
git sparse-checkout init --cone
git sparse-checkout set specs/ docs/ rfcs/ adrs/
git checkout origin/main

# Review worktree
git worktree add ~/.draft/worktrees/{repo-id}/reviews draft/reviews
```

### Publish changes

```bash
cd ~/.draft/worktrees/{repo-id}/reviews
git add .
git commit -m "Add comments on specs/authentication.md"
git push origin draft/reviews
```

### Sync

```bash
# Fetch all
git fetch origin main draft/reviews

# Update document worktree
cd ~/.draft/worktrees/{repo-id}/docs
git checkout origin/main

# Update review worktree
cd ~/.draft/worktrees/{repo-id}/reviews
git merge --ff-only origin/draft/reviews
```
