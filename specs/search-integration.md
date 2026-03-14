---
title: Search Integration
description: Wire project search into /spec, /refine, and /implement skills for automatic indexing and context retrieval
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: search-integration

Source: [docs/project-search.md](../docs/project-search.md)

Depends on: [search-indexer](search-indexer.md), [search-query](search-query.md)

## Goal

Integrate project search into existing draft workflows: auto-index after spec writes, auto-search before implementation. Agents get relevant codebase context without manual grep exploration.

## Acceptance Criteria

- [x] `/spec` triggers `draft index` (incremental) after writing the spec file
- [x] `/refine` triggers `draft index` (incremental) after writing the spec file
- [x] `/implement` runs `draft search` with the spec's title and key terms before generating code, includes top results as agent context
- [x] Indexing side-effects are silent (no agent-visible output unless errors)
- [x] Search results in `/implement` are formatted as context the implementing agent can use

## Approach

Add post-write step to `skills/spec.md` and `skills/refine.md`: after the spec file is written, run `draft index`. This is lightweight (~50ms for incremental, only the changed spec re-indexes).

Add pre-generation step to `skills/implement.md`: read spec title and acceptance criteria, extract key terms (nouns, technical terms — simple heuristic or use title as-is for MVP), run `draft search "<terms>" --limit 10`, inject top results into the implementing agent's context prompt.

## Affected Modules

- `skills/spec.md` — add post-write instruction: run `draft index`
- `skills/refine.md` — add post-write instruction: run `draft index`
- `skills/implement.md` — add pre-generation step: extract terms from spec, run `draft search`, include results as context

## Test Strategy

- **Spec integration**: run `/spec`, verify `draft index` runs after spec file written. Search for new spec content, verify found.
- **Refine integration**: run `/refine`, verify `draft index` runs after spec updated.
- **Implement integration**: create spec with known terms, run `/implement`, verify search results appear in agent context before code generation starts.

## Out of Scope

- `/review` triggering index updates
- Git hook integration (post-checkout)
- Key term extraction beyond simple title/criteria parsing
- Automatic staleness detection or age-based re-indexing
