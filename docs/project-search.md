---
title: Project Search (MVP)
description: FTS5 + trigram indexed search over the full codebase, integrated into existing draft commands
status: proposed
author: heiko-braun
---

# Feature: project-search

## Goal

Give agents fast, ranked, token-efficient search over the entire project.
Index everything — source, specs, docs, config — and wire it into the
commands agents already use: `draft search` for direct queries, automatic
indexing after `/spec` and `/refine`, automatic search during `/implement`.

## Motivation

Agents exploring a project rely on ripgrep or file reads. This returns
unranked, full-line matches and wastes tokens on irrelevant context.
An FTS5 index with BM25 ranking surfaces the most relevant hits first,
keeps responses compact, and enables natural language queries like
"where is rate limiting handled?" that grep can't answer.

Two specific pain points this solves:

1. **During implementation**: The agent needs to understand existing code
   before writing new code. Today that's a guess-and-grep loop. With an
   index, `/implement` can automatically pull relevant context from the
   codebase before the agent starts writing.

2. **During spec authoring**: Before writing a new spec, agents should
   check for overlap, conflicts, or prior decisions. With an index,
   `/spec` can surface related specs and source files as context.

## Acceptance Criteria

- [ ] `draft index` builds/updates a SQLite database with two FTS5 indexes (porter-stemmed + trigram) from the full project tree
- [ ] Indexing is incremental: only files whose content has changed are re-indexed
- [ ] `draft search <query>` returns ranked results with file path, line range, and snippet
- [ ] Search uses two FTS5 backends (porter-stemmed for natural language, trigram for substrings) with weighted score merging
- [ ] `.gitignore` patterns are respected; binary files and the index itself are skipped
- [ ] `/spec` and `/refine` trigger `draft index` after writing the spec file
- [ ] `/implement` runs `draft search` with the spec's title and key terms before generating code
- [ ] The index covers the full project tree, not just specs or source
- [ ] The index is stored outside the project tree (cache directory, keyed by project path)
- [ ] First full index of a ~10k LOC project completes in under 2 seconds
- [ ] No MCP server required — agents invoke `draft` CLI directly

## Non-Goals (for MVP)

- Symbol extraction, outline, or reference tools
- Semantic or vector search
- File watching or background re-indexing
- MCP server exposure (agents call `draft index` and `draft search` via bash)

## Approach

### Architecture

Two components: an indexer and a searcher, both callable from the CLI
and from within other draft commands.

```
┌─────────────────────────────────────┐
│         Existing Commands           │
│                                     │
│  /spec ──── index after write       │
│  /refine ── index after write       │
│  /implement ── search before gen    │
└──────────────┬──────────────────────┘
               │ calls
┌──────────────▼──────────────────────┐
│           CLI Commands              │
│  draft index [--force]              │
│  draft search <query> [--limit]     │
│  draft search --status              │
└──────────────┬──────────────────────┘
               │ uses
┌──────────────▼──────────────────────┐
│         Search Library              │
│                                     │
│  Indexer: walk, hash, upsert        │
│  ┌────────────┐  ┌───────────────┐  │
│  │ fts        │  │ fts_trigram   │  │
│  │ porter     │  │ trigram       │  │
│  │ stemming   │  │ substrings   │  │
│  │ BM25 rank  │  │ BM25 rank    │  │
│  └─────┬──────┘  └──────┬────────┘  │
│        └───────┬─────────┘          │
│          Score Merger               │
│   (weighted combination + dedup)    │
│                                     │
│  Store: SQLite via mattn/go-sqlite3 │
└─────────────────────────────────────┘
```

### Database Schema

Single SQLite database with an index metadata table, a file tracking
table, and two FTS5 virtual tables (one for natural language, one for
substring matching):

```sql
-- Index metadata: stores the project root so --prune and --list
-- can map database files back to their projects without reversing
-- the hash
CREATE TABLE index_meta (
    key      TEXT PRIMARY KEY,
    value    TEXT NOT NULL
);
-- Populated on creation:
--   ('project_root', '/Users/heiko/src/draft')
--   ('created_at',   '2026-03-14T10:23:41Z')
--   ('schema_version', '1')

-- Track indexed files for incremental updates
CREATE TABLE files (
    id       INTEGER PRIMARY KEY,
    path     TEXT UNIQUE NOT NULL,
    hash     TEXT NOT NULL,       -- xxh3 content hash (hex)
    mtime    INTEGER NOT NULL,    -- unix timestamp, fast-path skip
    indexed  INTEGER NOT NULL     -- unix timestamp of last index
);

-- Full-text index: natural language queries, BM25 ranking
-- Porter stemming so "handling" matches "handler"
CREATE VIRTUAL TABLE fts USING fts5(
    path,
    content,
    content_rowid,               -- links to files.id
    tokenize = 'porter unicode61'
);

-- Trigram index: substring and pattern matching
-- Contentless to avoid storing file content twice
-- detail='none' for minimal index footprint
CREATE VIRTUAL TABLE fts_trigram USING fts5(
    path,
    content,
    content = '',                -- contentless: no duplicate storage
    content_rowid = id,          -- points to files.id for joins
    tokenize = 'trigram',
    detail = 'none'
);
```

**Why two FTS5 tables:**

- `fts` uses Porter stemming to handle natural language queries
  ("authentication middleware", "rate limiting"). Stemming conflates
  word forms, which is good for recall but can't do exact substring
  matching.
- `fts_trigram` uses FTS5's built-in trigram tokenizer to handle
  substring queries ("CfgLoader", "thHandler"), LIKE patterns, and
  partial identifiers. No custom trigram code needed — FTS5 handles
  trigram decomposition, candidate intersection, and false-positive
  filtering internally.

Both are populated from the same file content during indexing — one
INSERT into `fts`, one INSERT into `fts_trigram` per file. The
`content=''` option on `fts_trigram` means the trigram index stores
only the inverted index, not a second copy of the text. The
`detail='none'` option omits per-token position data, roughly halving
trigram index storage with no impact on query performance.

One index for everything. No scope columns, no type distinctions.
Specs, source, docs, config — it all goes in. The agent's query
determines what's relevant, not a pre-assigned category.

### Incremental Indexing

```
1. Walk project tree (respecting .gitignore)
2. For each file:
   a. stat() for mtime — if unchanged vs files table, skip (fast path)
   b. If mtime differs, read file, compute xxh3(content)
   c. If hash matches stored hash, update mtime only (no re-index)
   d. If hash differs:
      - Delete old rows from fts and fts_trigram for this file_id
      - Insert new row into fts (path, content)
      - Insert new row into fts_trigram (path, content)
      - Update files row with new hash
3. Delete rows for paths no longer on disk (files + both FTS5 tables)
4. PRAGMA optimize
```

mtime as fast path, xxh3 as truth. The common case (nothing changed)
touches no file content at all.

Indexing a file is two INSERTs into FTS5 virtual tables — no custom
trigram extraction, no bulk-insert of trigram rows. FTS5 handles
tokenization internally for both tables. The `fts_trigram` table is
contentless (`content=''`), so the second INSERT stores only the
inverted trigram index, not a second copy of the file content.

### File Filtering

| Rule                          | Action  |
|-------------------------------|---------|
| Matches `.gitignore`          | Skip    |
| Binary file (null bytes)      | Skip    |
| File > 1 MB                   | Skip    |
| `.git/`, `node_modules/`, etc | Skip    |
| Index database itself         | Skip    |
| Everything else               | Index   |

Binary detection: read the first 8192 bytes, check for null bytes.
Simple, fast, and correct for the vast majority of cases.

### Search

```bash
draft search "authentication middleware" --limit 10
```

#### Query Routing

The searcher classifies each query and routes it to the appropriate
backend(s):

| Query shape                    | Example                  | Backend            |
|--------------------------------|--------------------------|--------------------|
| Natural language (words)       | `authentication flow`    | `fts` only         |
| Substring / partial identifier | `CfgLoader`              | `fts_trigram` only |
| Mixed / ambiguous              | `AuthHandler`            | Both → merge       |

Classification heuristic: if the query contains spaces or common
English words, route to FTS5. If it looks like a code identifier
(camelCase, snake_case, no spaces, contains uppercase mid-word),
route to trigram. When uncertain, run both and merge.

#### `fts` Backend (Natural Language)

```sql
SELECT f.path,
       snippet(fts, 1, '»', '«', '…', 32) as snippet,
       bm25(fts, 5.0, 1.0) as score
FROM fts
JOIN files f ON f.id = fts.rowid
WHERE fts MATCH 'authentication middleware'
ORDER BY score
LIMIT ?;
```

The `bm25(fts, 5.0, 1.0)` weights path matches 5x higher than content
matches. A file named `auth_middleware.go` should rank above a file
that mentions authentication in a comment.

#### `fts_trigram` Backend (Substring)

For substring queries, FTS5's trigram tokenizer handles everything
internally — decomposition, candidate matching, and false-positive
filtering:

```sql
-- Substring match: finds "CfgLoader" anywhere in file content
SELECT f.path,
       bm25(fts_trigram, 5.0, 1.0) as score
FROM fts_trigram
JOIN files f ON f.id = fts_trigram.rowid
WHERE fts_trigram MATCH 'CfgLoader'
ORDER BY score
LIMIT ?;
```

The trigram table also supports LIKE and GLOB patterns directly,
which is useful for wildcard searches:

```sql
-- Pattern match via trigram index
SELECT f.path
FROM fts_trigram
JOIN files f ON f.id = fts_trigram.rowid
WHERE content LIKE '%AuthMiddle%';
```

No custom trigram extraction, no IDF scoring, no verification step.
FTS5 does it all. The `content=''` option means the trigram table
doesn't store file content, but FTS5 still indexes the trigrams at
insert time and uses them for matching.

Note: because `fts_trigram` is contentless, `snippet()` and
`highlight()` are not available on it. Snippets come from the `fts`
table or from reading the file directly.

#### Score Merging

When both backends run, results are merged by file path (deduped)
with weighted scores:

```
final_score = (w_fts × norm(fts_score)) + (w_tri × norm(trigram_score))
```

Default weights: `w_fts = 0.6`, `w_tri = 0.4`. FTS5 gets more weight
because Porter-stemmed BM25 is a stronger relevance signal for most
queries. Trigram scores fill in the gaps for substring matches that
FTS5 misses entirely.

Both backends now produce BM25 scores, so normalization is
straightforward — min-max normalize each result set to [0, 1] before
combining.

#### Output Format

Token-efficient, one result per block:

```
src/auth/middleware.go:42-58 (score: 0.87)
  …validates the »authentication middleware« chain before…

specs/auth-flow.md:1-15 (score: 0.74)
  …describes the »authentication middleware« integration…

config/routes.yaml:12-14 (score: 0.31)
  …mounts »authentication« handler on /api…
```

### Command Integration

#### After `/spec` and `/refine`

When a spec file is written or updated, trigger an incremental index.
This is lightweight — only the changed spec file will be re-indexed
(hash changed), everything else hits the mtime fast path and skips.

```
/spec
  └── writes specs/{slug}.md
       └── runs: draft index (incremental, ~50ms)
```

The agent doesn't see this. It's a side effect of the write.

#### Before `/implement`

When implementation starts, the spec content is used to generate
search queries. The implement skill:

1. Reads the spec's title and acceptance criteria
2. Extracts key terms (nouns, technical terms)
3. Runs `draft search` with those terms
4. Includes the top results as context for the implementing agent

```
/implement auth-flow
  └── reads specs/auth-flow.md
       └── extracts: "authentication", "OAuth", "token refresh"
            └── runs: draft search "authentication OAuth token refresh" --limit 10
                 └── top results added to agent context
```

This replaces the agent's manual grep exploration with a single
ranked search that gives it the most relevant existing code upfront.

### Index Management

draft maintains one SQLite database per project. The index is stored
outside the project tree so it never appears in version control, never
interferes with builds, and survives project directory renames as long
as the absolute path doesn't change.

#### Project Identification

Each project is identified by the **resolved** absolute path to its
root directory. Symlinks are resolved before hashing, so the same
project accessed via different symlinks always maps to the same index.

The root is determined by walking upward from the current directory
looking for a `.draft/` directory or a `CLAUDE.md` file — the same
heuristic draft already uses for project detection. If neither is
found, the current working directory is the root.

The index filename is derived from the project root:

```
index_path = <cache_dir>/draft/<xxh3_hex(realpath(project_root))>.db
```

For example, `/Users/heiko/src/draft` might hash to `a3f7c1d2e9b04856`,
producing `~/Library/Caches/draft/a3f7c1d2e9b04856.db`.

#### Schema Versioning

The `index_meta` table stores a `schema_version` key. On startup,
draft compares the stored version against its expected version:

- **Match**: proceed normally.
- **Older version**: run migration (alter tables, rebuild if needed).
- **Newer version** (downgrade): refuse to open, print error suggesting
  `draft index --force` to rebuild.
- **Missing or corrupt**: treat as first run, create fresh.

#### Storage Location

Platform-appropriate cache directories, following OS conventions:

| Platform | Base directory                         | Example                                              |
|----------|----------------------------------------|------------------------------------------------------|
| macOS    | `~/Library/Caches/draft/`              | `~/Library/Caches/draft/a3f7c1d2e9b04856.db`        |
| Linux    | `${XDG_CACHE_HOME:-~/.cache}/draft/`   | `~/.cache/draft/a3f7c1d2e9b04856.db`                |

The cache directory is created on first `draft index` if it doesn't
exist. The `--db` flag overrides the computed path to a specific file,
useful for CI or testing where the cache directory may not be writable.

#### Index Lifecycle

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  No index    │────▶│  Created     │────▶│  Current     │
│  (first run) │     │  (full scan) │     │  (in sync)   │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                 │
                          file changed ──────────┤
                          file deleted ──────────┤
                          file added   ──────────┤
                                                 ▼
                                          ┌──────────────┐
                                          │  Stale       │
                                          │  (drift)     │
                                          └──────┬───────┘
                                                 │
                                     draft index │
                                                 ▼
                                          ┌──────────────┐
                                          │  Current     │
                                          │  (in sync)   │
                                          └──────────────┘
```

**Creation**: On first `draft index`, the database file and schema are
created, and a full scan of the project tree populates both FTS5 tables
and the `files` metadata table. This is the slowest operation — typically
1-2 seconds for a 10k LOC project.

**Incremental update**: Subsequent `draft index` runs walk the tree,
compare mtimes and content hashes, and only re-index changed files.
For a project with no changes this takes ~50ms (stat calls only, no
file reads).

**Forced rebuild**: `draft index --force` drops and recreates all tables,
then does a full scan. Use when the index is suspected to be corrupt or
after significant project restructuring (e.g., large merge, directory
renames).

**Staleness**: There is no automatic invalidation. The index becomes
stale whenever files change outside of draft's workflow (e.g., manual
edits, `git pull`, IDE refactors). This is acceptable because:
- `/spec` and `/refine` re-index after writing
- `/implement` can re-index before searching if the index is old
- The agent can always run `draft index` explicitly

#### Status and Diagnostics

`draft search --status` reports the current index state:

```
Index: ~/Library/Caches/draft/a3f7c1d2e9b04856.db
Project: /Users/heiko/src/draft
Files indexed: 347
Last indexed: 2026-03-14 10:23:41 (12 minutes ago)
Database size: 2.1 MB
```

This gives the agent (or human) a quick check on whether the index
is fresh enough to trust, without having to understand where the
database lives.

#### Multiple Projects

A developer working on several projects gets one database file per
project, all living in the same cache directory:

```
~/.cache/draft/
├── a3f7c1d2e9b04856.db   # /home/heiko/src/draft
├── 7e2b9f1c4a6d8032.db   # /home/heiko/src/other-project
└── d1c5a8f3b7e24690.db   # /home/heiko/src/client-work
```

No registry, no config file, no global state. The mapping from project
to index is pure function: `path → hash → filename`. If the project
is deleted, the index file becomes orphaned but harmless.

#### Cleanup

Orphaned index files (for projects that no longer exist) can be cleaned
up with `draft index --prune`. This scans the cache directory, reads
the `project_root` from each database's `index_meta` table, checks
whether that path still exists on disk, and deletes indexes whose
projects are gone. This is a manual, opt-in operation — not automatic —
because a project directory being temporarily unmounted or on a
different branch shouldn't trigger deletion.

`draft index --list` shows all known indexes:

```
PATH                              FILES   SIZE    LAST INDEXED
/Users/heiko/src/draft              347   2.1 MB  12 min ago
/Users/heiko/src/other-project      892   5.7 MB  3 days ago
/Users/heiko/src/client-work       1204   8.3 MB  2 weeks ago
```

This reads `index_meta` from each `.db` file in the cache directory.
No global registry is needed.

### Key Dependencies

| Dependency                | Purpose                          |
|---------------------------|----------------------------------|
| `mattn/go-sqlite3`        | SQLite with FTS5 (CGo, bundled)  |
| `zeebo/xxh3`              | Fast content hashing             |
| `go-git/go-git` (ignore)  | .gitignore pattern matching      |

Build tag `fts5` required for `mattn/go-sqlite3` to enable FTS5.

## Affected Modules

- `cmd/index.go` — new `draft index` command (including `--force`, `--prune`, `--list`)
- `cmd/search.go` — new `draft search` command (including `--status`)
- `internal/search/project.go` — project root detection, path-to-hash mapping, cache directory resolution
- `internal/search/indexer.go` — file walker, hasher, dual FTS5 upsert logic
- `internal/search/searcher.go` — query classification, backend dispatch, score merging
- `internal/search/store.go` — SQLite connection, schema creation/migration, forced rebuild
- `skills/spec.md` — add post-write hook: run `draft index`
- `skills/refine.md` — add post-write hook: run `draft index`
- `skills/implement.md` — add pre-generation step: run `draft search` with spec terms

## Test Strategy

### Indexing
- **Full index**: Create temp project with mixed files (Go, Markdown, YAML,
  binary, `.gitignore`d files). Index. Verify correct files are in both
  `fts` and `fts_trigram` tables, excluded files are not.
- **Incremental update**: Index, modify one file, re-index. Verify only
  that file's rows changed in both FTS5 tables (check `indexed` timestamp).
- **Incremental delete**: Index, delete a file, re-index. Verify removed
  from `files`, `fts`, and `fts_trigram` tables.
- **mtime-only change**: Index, touch a file (same content, new mtime),
  re-index. Verify no FTS5 rows were re-inserted in either table.
- **git checkout edge case**: Index, simulate `git checkout` (content
  changes, mtime may or may not update). Verify hash-based detection
  catches the change.
- **Binary skip**: Include a binary file (e.g., .png), verify it's excluded.
- **Large file skip**: Include a file > 1 MB, verify it's excluded.
- **Forced rebuild**: Index, then `draft index --force`. Verify all
  `indexed` timestamps are refreshed (full re-scan, not incremental).
- **Contentless trigram table**: Verify `fts_trigram` does not store
  file content (SELECT content FROM fts_trigram returns NULL).

### Search
- **FTS5 ranking**: Insert known documents with varying relevance.
  Query with natural language, verify BM25 ordering matches expected order.
- **Path weighting**: Search for a term that appears in both a filename
  and a file body. Verify the filename match ranks higher.
- **Trigram substring match**: Index files containing `AppCfgLoader` and
  `UserConfigService`. Search for `CfgLoad`. Verify the first file is
  found via `fts_trigram`, the second is not.
- **Trigram LIKE pattern**: Index files, search using
  `WHERE content LIKE '%AuthMiddle%'` against `fts_trigram`. Verify
  correct matches are returned.
- **Trigram minimum length**: Search for a 2-character string. Verify
  it falls back gracefully (FTS5 trigram requires 3+ characters).
- **Query routing**: Verify `"error handling"` routes to `fts` only,
  `CfgLoader` routes to `fts_trigram` only, and `AuthHandler` routes
  to both.
- **Score merging**: Insert files that rank differently in `fts` vs
  `fts_trigram`. Search with a query that hits both backends. Verify
  merged ranking reflects the configured weights (0.6 / 0.4).
- **Snippet source**: Verify snippets come from the `fts` table (not
  `fts_trigram`, which is contentless and can't produce snippets).

### Index Management
- **Project root detection**: Create a temp directory with `.draft/`
  marker at the root. Run `draft index` from a subdirectory. Verify the
  index covers the full project tree, not just the subdirectory.
- **Project root fallback**: Run `draft index` in a directory with no
  `.draft/` or `CLAUDE.md` marker. Verify the current directory is used
  as root.
- **Deterministic hashing**: Run `draft index` twice on the same project.
  Verify both runs use the same database file (same hash of project path).
- **Multi-project isolation**: Index two different projects. Verify each
  gets its own database file. Search in project A returns no results
  from project B.
- **Cache directory creation**: Remove the cache directory, run
  `draft index`. Verify the directory is created automatically.
- **--db override**: Run `draft index --db /tmp/custom.db`. Verify the
  index is written to that path, not the default cache location.
- **Status output**: Index a project, run `draft search --status`. Verify
  it reports correct file count, project path, last-indexed time, and
  database size.
- **Prune**: Index two projects. Delete one project's directory. Run
  `draft index --prune`. Verify the orphaned index is deleted, the
  other is kept.
- **Prune safety**: Index a project, unmount or rename the directory.
  Run `draft index --prune`. Verify the index is deleted (path no longer
  exists). Re-create the directory and re-index — a new database file
  is created (same hash, fresh content).
- **Symlink resolution**: Create a project accessed via symlink. Verify
  the index uses the resolved (real) path, so the same project accessed
  via different symlinks shares one index.
- **Schema version match**: Open an index with matching schema_version.
  Verify it proceeds normally without rebuild.
- **Schema version mismatch (upgrade)**: Create an index with an older
  schema_version. Run `draft index`. Verify migration runs and version
  is updated.
- **Schema version mismatch (downgrade)**: Create an index with a newer
  schema_version. Run `draft index`. Verify it refuses to open and
  suggests `--force`.

### Command Integration
- **Spec integration**: Run `/spec`, verify index is updated. Check that
  the new spec is findable via `draft search`.
- **Implement integration**: Create a spec with known terms, run
  `/implement`, verify search results appear in the agent's context.

## Open Questions

1. **Tokenizer for code**: `porter unicode61` stems words in `fts`, which
   is great for natural language but means `Handler` and `handling`
   conflate. The `fts_trigram` table now covers exact substring matching,
   so Porter stemming may be fine — but should we offer an `--exact`
   flag that bypasses `fts` and goes trigram-only?

2. **Implement search extraction**: How should key terms be extracted
   from a spec for the pre-implementation search? Options: (a) use the
   title as-is, (b) extract nouns from acceptance criteria with a simple
   heuristic, (c) let the agent decide what to search for.

3. **Index on other commands**: Should `/review` also trigger an index
   update, or is spec/refine sufficient to keep the index fresh? What
   about a `draft index` in the project's post-checkout git hook?

4. **Score merge weights**: The default 0.6/0.4 FTS5/trigram split is a
   starting guess. Should this be tunable via config, or is it better
   to find the right default empirically and hardcode it?
