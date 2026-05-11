---
title: Search Output Format
description: Replace inline snippet format with markdown code-fenced output using language-appropriate syntax highlighting
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: search-output-format

Source: improves [search-query](search-query.md)

## Goal

Make `draft search` output directly useful as context for agents and humans. Replace the current flat snippet format with `path (score)` header followed by a fenced code block with language tag inferred from file extension. Keep `»«` markers for match highlighting.

## Acceptance Criteria

- [x] Each result rendered as `path (score: X.XX)` on its own line, followed by a fenced code block with language tag
- [x] Language tag inferred from file extension (`.go` → `go`, `.md` → `markdown`, `.yaml`/`.yml` → `yaml`, etc.); unknown extensions use no tag
- [x] `»«` match markers preserved in snippet content; `…` truncation marker preserved
- [x] Trigram-only results (no snippet) show path and score only, no empty code block

## Approach

Update `FormatResults` to emit `path (score: X.XX)\n` + fenced block with language tag from a `langFromExt(path)` helper. Add extension-to-language map covering common project file types. FTS5 `snippet()` call unchanged — already produces the right content with `»«` markers.

## Affected Modules

- `internal/search/searcher.go` — update `FormatResults`, add `langFromExt` helper
- `internal/search/searcher_test.go` — update format tests

## Test Strategy

- **Format with snippet**: result with `.go` path produces ` ```go ` fenced block containing snippet
- **Format with markdown file**: `.md` path produces ` ```markdown ` block
- **Format unknown extension**: unknown ext produces bare ` ``` ` block
- **Format without snippet**: trigram-only result (empty snippet) renders path and score only
- **No results**: still returns `"No results found.\n"`
- **Markers preserved**: output contains `»` and `«` around matched terms

## Out of Scope

- Line numbers or line ranges in output
- Syntax highlighting beyond language tag
- Changes to score merging or ranking logic
