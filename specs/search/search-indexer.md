---
title: Search Indexer
description: Incremental file walker and dual FTS5 indexer with draft index CLI command
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: search-indexer

Source: [docs/project-search.md](../docs/project-search.md)

Depends on: [search-store](search-store.md)

## Goal

Walk the project tree, hash file content, and maintain both FTS5 indexes incrementally. Expose via `draft index` CLI command with `--force`, `--prune`, `--list`, and `--db` flags. First full index of ~10k LOC project under 2 seconds.

## Acceptance Criteria

- [x] `draft index` builds/updates both FTS5 indexes from the full project tree
- [x] Incremental: mtime fast-path skips unchanged files; xxh3 hash detects content changes when mtime differs
- [x] mtime-only changes (same content, new mtime) update `files.mtime` without FTS5 re-insert
- [x] Deleted files are removed from `files`, `fts`, and `fts_trigram`
- [x] `.gitignore` patterns respected; binary files (null bytes in first 8192 bytes), files > 1 MB, `.git/`, `node_modules/`, and the index db itself are skipped
- [x] `--force` drops and recreates all tables, does full re-scan
- [x] `--prune` scans cache dir, deletes indexes whose `project_root` no longer exists on disk
- [x] `--list` shows all indexes with project path, file count, size, last-indexed time
- [x] First full index of ~10k LOC project completes in under 2 seconds

## Approach

`internal/search/indexer.go`: walk project tree using `filepath.WalkDir`, filter via `go-git/go-git` gitignore matching plus hardcoded skips (binary, size, dotdirs). For each file: stat for mtime, compare against `files` table; if mtime changed, read + xxh3 hash; if hash changed, delete old FTS5 rows, insert into both `fts` and `fts_trigram`, update `files` row. After walk, delete `files` rows for paths no longer on disk (cascade to FTS5). Wrap full index in a transaction. `PRAGMA optimize` at end.

`cmd/index.go`: Cobra command wiring `draft index [--force] [--prune] [--list] [--db]`. `--force` calls store's drop-and-recreate before indexing. `--prune` and `--list` operate on cache directory contents.

## Affected Modules

- `internal/search/indexer.go` — new: file walker, hasher, dual FTS5 upsert, incremental logic
- `cmd/index.go` — new: `draft index` command with flags
- `go.mod` — add `go-git/go-git` (gitignore matching)

## Test Strategy

- **Full index**: temp project with Go, Markdown, YAML, binary, `.gitignore`d files. Verify correct files in both FTS5 tables, excluded files absent.
- **Incremental update**: index, modify one file, re-index. Verify only that file's rows changed (check `indexed` timestamp).
- **Incremental delete**: index, delete a file, re-index. Verify removed from all three tables.
- **mtime-only change**: index, touch file (same content), re-index. Verify no FTS5 re-insert.
- **Binary skip**: include .png, verify excluded.
- **Large file skip**: include file > 1 MB, verify excluded.
- **Forced rebuild**: index, then `--force`. Verify all `indexed` timestamps refreshed.
- **Contentless trigram**: verify `fts_trigram` does not store file content.
- **Prune**: index two projects, delete one's directory, run `--prune`. Verify orphaned index deleted, other kept.
- **List**: index projects, run `--list`. Verify output shows path, file count, size, last-indexed time.
- **Performance**: index a ~10k LOC project, verify completes under 2 seconds.

## Out of Scope

- Search queries, ranking, output (→ search-query)
- Integration with /spec, /refine, /implement (→ search-integration)
- File watching, background re-indexing
