---
title: Anchor System
description: Anchor comments to document locations with a resolution cascade that survives edits
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Anchor System

## Goal

Provide a robust anchoring mechanism that ties review threads to specific document locations and gracefully handles document evolution — paragraphs rewritten, sections moved, content added/removed.

## Acceptance Criteria

- [ ] Anchor data model: `heading_path`, `paragraph_index`, `excerpt`, `content_hash`, `char_range`, `source_ref`
- [ ] Resolution cascade implemented in order: exact match → structural match → fuzzy search → orphaned
- [ ] Exact match: content_hash matches paragraph at same heading_path and paragraph_index
- [ ] Structural match: heading_path exists, paragraph at/near index contains excerpt (substring match)
- [ ] Fuzzy search: heading_path changed; search all paragraphs for excerpt; re-anchor if strong match found
- [ ] Orphaned: no match — thread marked orphaned with original anchor preserved
- [ ] Re-anchoring updates the thread's anchor data (persisted via review-datamodel)

## Approach

New `internal/review/anchor.go`. The `ResolveAnchors(index *DocIndex, threads []Thread) []AnchorResult` function runs the cascade for each thread. Fuzzy matching uses normalized substring search (lowercase, collapsed whitespace) with a minimum match ratio. Re-anchored threads get updated anchor fields. Orphaned threads retain their original anchor for display context.

## Affected Modules

- `internal/review/anchor.go` (new) — anchor resolution cascade
- `internal/review/types.go` — `Anchor` struct, `AnchorResult` enum (exact/structural/fuzzy/orphaned)
- `internal/review/docindex.go` — consumed for heading tree and paragraph lookup

## Test Strategy

- Unit test: exact match — unchanged paragraph resolves immediately
- Unit test: structural match — paragraph content unchanged but index shifted
- Unit test: fuzzy match — section renamed, paragraph found elsewhere in doc
- Unit test: orphaned — paragraph deleted entirely, thread marked orphaned
- Unit test: excerpt matching with minor whitespace/punctuation changes

## Out of Scope

- Creating new anchors from user selections (UI responsibility)
- Persisting re-anchored data to disk (review-datamodel handles write)
- Diffing between anchor versions for display

## Notes

- Reference: docs/docs-review.md §7
- The resolution cascade runs on every sync when documents have changed
- Fuzzy matching should be conservative — better to orphan than mis-anchor
