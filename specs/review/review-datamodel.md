---
title: Review Data Model and Sync
description: CRUD operations for reviews, threads, comments, participants plus sync and conflict resolution
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review Data Model and Sync

## Goal

Implement the core data layer: reading and writing review objects (reviews, threads, comments, participants) as JSON files in the review worktree, plus the sync/publish/conflict-resolution workflow.

## Acceptance Criteria

- [ ] Review CRUD: create, read, update status, list open reviews, add/remove reviewers
- [ ] Thread CRUD: create with anchor, add comment, resolve/reopen, list by document
- [ ] Participant registration: auto-create from git config on first action, stored in `participants/{user-hash}.json`
- [ ] Batch-and-publish model: changes accumulate as uncommitted files; `Publish()` commits and pushes
- [ ] Sync: fetch remote review branch + fast-forward local; fetch documents and trigger re-index
- [ ] Conflict resolution: on push rejection, fetch + rebase; semantic merge for thread files (combine comment arrays by ID, dedup, order by timestamp, take latest status)
- [ ] Stale approval detection: approvals flagged stale when source_ref differs from latest document commit

## Approach

New `internal/review/store.go` for file-based CRUD against the review worktree. Each entity maps to a JSON file per the layout in §4. `internal/review/sync.go` handles fetch/push/rebase/semantic-merge. The semantic merge parses both JSON versions of a conflicting thread file, merges comment arrays, and writes the resolved file. Uses `git` CLI for all operations.

## Affected Modules

- `internal/review/store.go` (new) — CRUD operations, file path conventions, JSON serialization
- `internal/review/sync.go` (new) — fetch, publish (commit+push), conflict detection, semantic merge
- `internal/review/types.go` — `Review`, `Thread`, `Comment`, `Participant`, `ReviewerStatus` structs
- `internal/review/worktree.go` — consumed for worktree paths and update operations

## Test Strategy

- Unit test: serialize/deserialize each entity type, round-trip fidelity
- Unit test: semantic merge — two versions of a thread with non-overlapping new comments merge cleanly
- Unit test: semantic merge — conflicting status changes picks latest timestamp
- Integration test: create review + threads in temp repo, publish, verify committed files
- Unit test: stale approval detection when source_ref changes

## Out of Scope

- UI for displaying reviews (review-ui spec)
- Anchor resolution on sync (review-anchors spec)
- Real-time sync / websockets
- Notifications

## Notes

- Reference: docs/docs-review.md §8, §9
- Thread files keyed by `{document-path}/{thread-id}.json`
- Comment IDs should be UUIDs for deduplication during merge
