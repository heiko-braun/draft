---
title: Review Worktree Management
description: Create and manage git worktrees for document reading and review data access
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review Worktree Management

## Goal

Manage two git worktrees — one sparse checkout for documents, one for review data — so `draft review` can read/write review state without touching the user's working tree or checked-out branch.

## Acceptance Criteria

- [ ] Document worktree created at `~/.draft/worktrees/{repo-id}/docs` with sparse checkout of configured document paths
- [ ] Review worktree created at `~/.draft/worktrees/{repo-id}/reviews` tracking the `draft/reviews` branch
- [ ] `repo-id` derived deterministically from the repo's remote origin URL (hash or slug)
- [ ] On subsequent launches, existing worktrees are verified (valid git state); broken worktrees are re-created
- [ ] Worktree update functions: fetch + checkout latest for docs, fetch + fast-forward for reviews
- [ ] Live mode: detect local modifications in document paths and prefer user's working tree over document worktree

## Approach

New `internal/review/worktree.go`. Use `git worktree add` with `--no-checkout` for the sparse docs worktree, then configure sparse-checkout. Use `git worktree list --porcelain` to verify existing worktrees. Derive `repo-id` from SHA256 of the normalized remote URL (first 12 hex chars). Live mode detection via `git status --porcelain` filtered to configured document paths.

## Affected Modules

- `internal/review/worktree.go` (new) — worktree lifecycle (create, verify, update, remove)
- `internal/review/repo.go` (new) — repo detection, remote URL, repo-id derivation
- `internal/review/branch.go` — called to ensure `draft/reviews` exists before creating the review worktree

## Test Strategy

- Integration test: create temp repo, run worktree setup, verify directories exist and git state is correct
- Verify sparse-checkout only contains configured paths
- Verify broken worktree detection: corrupt the worktree directory, assert re-creation
- Verify live mode: modify a doc in working tree, assert it's detected

## Out of Scope

- Document content parsing (separate spec: review-docindex)
- Review data reading/writing (separate spec: review-datamodel)
- Cleanup on repo deletion (documented limitation — worktrees become stale)

## Notes

- Reference: docs/docs-review.md §5
- `~/.draft/` directory may not exist; create it on first use
