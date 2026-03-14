---
title: Search Store
description: SQLite database layer with FTS5 schema, project root detection, cache directory resolution, and schema versioning
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: search-store

Source: [docs/project-search.md](../docs/project-search.md)

## Goal

Provide the persistence layer for project search: SQLite database with dual FTS5 virtual tables (porter-stemmed + trigram), project root detection, deterministic path-to-hash mapping, platform-appropriate cache directory resolution, and schema versioning with migration support.

## Acceptance Criteria

- [x] `OpenStore(projectRoot)` creates/opens a SQLite database at the platform cache path derived from `xxh3(realpath(projectRoot))`
- [x] Schema includes `index_meta`, `files`, `fts` (porter unicode61), and `fts_trigram` (contentless) tables as specified — `detail=none` is incompatible with the trigram tokenizer (trigram matching relies on phrase queries which require `detail=full`)
- [x] `index_meta` stores `project_root`, `created_at`, and `schema_version` on first creation
- [x] Schema version check on open: matching version proceeds, older version triggers migration, newer version returns error suggesting `--force`, missing/corrupt creates fresh
- [x] `DetectProjectRoot(cwd)` walks upward looking for `.draft/` or `CLAUDE.md`, falls back to cwd
- [x] Symlinks are resolved before hashing so the same project via different symlinks shares one index
- [x] `--db` flag support: `OpenStore` accepts an optional override path via `IndexPath`
- [x] Cache directory is created automatically if it doesn't exist

## Approach

New package `internal/search/`. `project.go` handles root detection (walk upward for `.draft/` or `CLAUDE.md`) and cache path computation (`xxh3` hex hash of resolved absolute path). `store.go` manages SQLite connection via `modernc.org/sqlite` (CGo-free), creates schema on first open, checks `schema_version` on subsequent opens. Store exposes low-level CRUD for `files` table and insert/delete for both FTS5 tables — no indexing or search logic.

## Affected Modules

- `internal/search/project.go` — new: project root detection, path-to-hash, cache dir resolution
- `internal/search/store.go` — new: SQLite connection, schema DDL, version check/migration, CRUD methods
- `go.mod` — add `modernc.org/sqlite`, `zeebo/xxh3`

## Test Strategy

- **Project root detection**: temp dir with `.draft/` marker at root, run from subdirectory, verify root found. Repeat with no marker, verify cwd returned.
- **Deterministic hashing**: same project path produces same db filename across calls.
- **Symlink resolution**: project accessed via symlink maps to same hash as real path.
- **Schema creation**: open fresh db, verify all four tables exist with correct structure.
- **Schema version match**: open existing db with matching version, verify no rebuild.
- **Schema version upgrade**: db with older version triggers migration, version updated.
- **Schema version downgrade**: db with newer version returns error mentioning `--force`.
- **Cache dir creation**: remove cache dir, open store, verify dir created.
- **--db override**: open with explicit path, verify db at that path.

## Out of Scope

- File walking, content hashing, indexing logic (→ search-indexer)
- Query execution, ranking, output formatting (→ search-query)
- CLI commands (→ search-indexer, search-query)
- Semantic/vector search, file watching, MCP server
