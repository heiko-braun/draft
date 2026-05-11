---
title: Review CLI Subcommand
description: The draft review cobra command with flags for branch, sync, status, and direct file opening
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review CLI Subcommand

## Goal

Add `draft review` as a top-level subcommand that orchestrates initialization, sync, and UI launch — the single entry point for the review workflow.

## Acceptance Criteria

- [ ] `draft review` — initializes (if needed), syncs, launches browser UI on default branch
- [ ] `draft review --branch feature/x` — targets a specific source branch for documents
- [ ] `draft review specs/auth.md` — opens UI directly to a specific document
- [ ] `draft review --sync` — fetches latest review data without opening UI (headless/CI)
- [ ] `draft review --status` — prints summary of open reviews and pending local changes to stdout
- [ ] Proper error messages when not in a git repo or when remote is unreachable

## Approach

New `internal/cli/review.go` following existing cobra command pattern (see `present.go`, `search.go`). The `RunE` function orchestrates: detect git root → ensure review branch exists → ensure worktrees → sync → launch server + open browser (or print status). Flags registered via cobra's `Flags()`. Reuses `openBrowser()` pattern from `present.go`.

## Affected Modules

- `internal/cli/review.go` (new) — cobra command definition, flag parsing, orchestration
- `internal/cli/root.go` — add `newReviewCmd()` registration
- `internal/review/` — all review packages consumed here as the integration point

## Test Strategy

- Unit test: flag parsing — verify all flag combinations produce correct config
- Integration test: `draft review --status` in a repo with review data prints expected output
- Integration test: `draft review --sync` fetches without launching browser
- Error test: running outside git repo produces clear error message

## Out of Scope

- HTTP server implementation (review-ui spec)
- Frontend assets (review-ui spec)
- Review data manipulation (review-datamodel spec)

## Notes

- Reference: docs/docs-review.md §3
- Port selection should follow same pattern as `present.go` (default port with `--port` override)
