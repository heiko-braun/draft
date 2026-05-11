---
title: Review Branch Initialization
description: Create and manage the draft/reviews orphan branch for storing review data
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review Branch Initialization

## Goal

Provide the foundational git infrastructure for `draft review` — an orphan branch (`draft/reviews`) that stores all review data (threads, comments, approvals) without polluting code branch history.

## Acceptance Criteria

- [ ] `initReviewBranch()` creates an orphan branch `draft/reviews` with initial structure: `schema-version`, `config.json`, empty `threads/`, `reviews/`, `participants/` directories
- [ ] If `draft/reviews` already exists (local or remote), initialization is skipped gracefully
- [ ] `schema-version` file contains `1` (integer, for future migrations)
- [ ] `config.json` is initialized with default document paths (`specs/`, `docs/`, `rfcs/`, `adrs/`), file patterns (`*.md`, `*.mdx`), and default branch (`main`)
- [ ] The branch is pushed to `origin` after creation

## Approach

New package `internal/review/branch.go`. Shell out to `git` (consistent with existing Draft patterns — see `present.go` using `exec.Command`). Create orphan branch using a **temporary worktree** (to avoid disturbing the main working tree), write initial files, commit with `--no-verify` (data-only commit, no Go code), push, then remove the temp worktree. Detect existing branch by checking `git rev-parse --verify draft/reviews` or `git ls-remote`.

## Affected Modules

- `internal/review/branch.go` (new) — orphan branch creation, existence detection, schema versioning
- `internal/review/config.go` (new) — config.json struct and serialization

## Test Strategy

- Unit test: verify `config.json` serialization matches expected schema
- Integration test: run init against a temp git repo, assert branch exists with correct file layout
- Idempotency test: run init twice, assert no error and no duplicate commits

## Out of Scope

- Worktree creation (separate spec: review-worktree)
- Schema migration logic (v1 only, migration deferred)
- Review data CRUD operations
- Remote authentication setup (inherits user's git credentials)

## Notes

- Reference implementation in docs/docs-review.md §4 and Appendix A
- The orphan branch pattern is similar to `gh-pages` — well understood by git tooling
- CRITICAL: Must use a temporary worktree for orphan creation to avoid `git clean -fd` destroying untracked files in the user's working tree
