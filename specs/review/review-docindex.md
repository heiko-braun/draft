---
title: Document Indexing
description: Scan, parse, and index markdown documents for structural navigation and anchor resolution
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Document Indexing

## Goal

Build an in-memory index of markdown documents that powers structural navigation, front-matter display, and provides the paragraph-level addressing needed by the anchor system.

## Acceptance Criteria

- [ ] Scan configured document paths and discover all `.md`/`.mdx` files
- [ ] Extract YAML front-matter (title, description, status, author) when present; fall back to first heading for title
- [ ] Build heading tree: hierarchy of `#`/`##`/etc. with text, nesting level, and byte offsets
- [ ] Map paragraphs to their containing heading section with start/end positions and paragraph index
- [ ] Compute per-paragraph content hash (SHA256 of normalized text)
- [ ] Index is ephemeral — rebuilt on launch/sync, not persisted

## Approach

New `internal/review/docindex.go`. Use `goldmark` (already a dependency — see `present.go`) to parse markdown into AST. Walk the AST to extract headings, paragraph boundaries, and text content. Front-matter extraction via `gopkg.in/yaml.v3` (also already used). Expose `IndexDocuments(root string, paths []string) (*DocIndex, error)` as the public API.

## Affected Modules

- `internal/review/docindex.go` (new) — scanner, parser, index builder
- `internal/review/frontmatter.go` (new) — YAML front-matter extraction (reusable beyond review)
- `internal/review/types.go` (new) — `DocIndex`, `Document`, `HeadingNode`, `Paragraph` structs

## Test Strategy

- Unit test: parse a known markdown file, assert heading tree structure matches expected
- Unit test: verify paragraph boundaries and content hashes for a doc with multiple sections
- Unit test: front-matter extraction with and without YAML block
- Unit test: file discovery across multiple configured paths with glob patterns
- Edge cases: empty files, files with only front-matter, deeply nested headings

## Out of Scope

- Anchor resolution logic (separate spec: review-anchors)
- Persistent caching / SQLite (v1 rebuilds on each launch)
- Rendering markdown to HTML (handled by the UI layer)
- Watching filesystem for changes (manual sync only in v1)

## Notes

- Reference: docs/docs-review.md §6
- The heading tree + paragraph map is the foundation for the anchor system
