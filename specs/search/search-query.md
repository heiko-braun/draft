---
title: Search Query
description: Query classification, dual FTS5 backend dispatch, BM25 score merging, and draft search CLI command
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: search-query

Source: [docs/project-search.md](../docs/project-search.md)

Depends on: [search-store](search-store.md), [search-indexer](search-indexer.md)

## Goal

Provide ranked, token-efficient search over the project index. Classify queries, route to appropriate FTS5 backend(s), merge scores, and return compact results with snippets. Expose via `draft search <query>` CLI command.

## Acceptance Criteria

- [x] `draft search <query>` returns ranked results with file path, line range, and snippet
- [x] Query routing: natural language (spaces, common words) → `fts` only; code identifiers (camelCase, snake_case, no spaces) → `fts_trigram` only; ambiguous → both with merge
- [x] FTS5 backend uses `bm25(fts, 5.0, 1.0)` weighting path matches 5x over content
- [x] Trigram backend handles substring matches; queries < 3 chars fall back gracefully
- [x] When both backends run, scores are min-max normalized to [0,1] then merged with weights 0.6 (fts) / 0.4 (trigram), deduplicated by file path
- [x] `--limit N` controls max results (default 20)
- [x] `--status` reports index path, project root, file count, last-indexed time, db size
- [x] Snippets use `»` / `«` markers, sourced from `fts` table (not contentless trigram)

## Approach

`internal/search/searcher.go`: classify query (regex heuristics for camelCase/snake_case/spaces), dispatch to one or both backends via SQL queries against `fts` and `fts_trigram`. Each backend returns `(path, score, snippet?)`. Merge: min-max normalize each result set, combine with weighted sum, dedup by path keeping highest score, sort descending, apply limit.

`cmd/search.go`: Cobra command `draft search <query> [--limit N] [--status]`. `--status` reads `index_meta` and `files` count, prints diagnostics. Default mode formats results as `path:lineRange (score: X.XX)` with indented snippet.

## Affected Modules

- `internal/search/searcher.go` — new: query classifier, backend dispatch, score merger, result formatting
- `cmd/search.go` — new: `draft search` command with `--limit`, `--status`

## Test Strategy

- **FTS5 ranking**: known documents with varying relevance. Natural language query, verify BM25 ordering.
- **Path weighting**: term in filename and body — filename match ranks higher.
- **Trigram substring**: index `AppCfgLoader` and `UserConfigService`, search `CfgLoad`. Verify first found, second not.
- **Trigram min length**: 2-char search falls back gracefully (no crash, no results or warning).
- **Query routing**: `"error handling"` → fts only, `CfgLoader` → trigram only, `AuthHandler` → both.
- **Score merging**: files ranking differently in each backend. Merged result reflects 0.6/0.4 weights.
- **Snippet source**: snippets come from `fts` table, not `fts_trigram`.
- **Status output**: verify reports correct file count, project path, last-indexed time, db size.
- **Limit**: verify `--limit 3` returns at most 3 results.

## Out of Scope

- Indexing logic (→ search-indexer)
- Tunable merge weights via config (hardcoded for MVP)
- `--exact` flag for trigram-only mode
- Semantic/vector search
