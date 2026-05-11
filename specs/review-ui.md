---
title: Review UI
description: Local HTTP server and browser frontend for document browsing, annotation, and threaded discussion
status: proposed
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review UI

## Goal

Deliver the interactive review experience: a local HTTP server serving a browser-based UI with document browsing, inline annotation, and threaded discussion — the visual layer on top of the review data model.

## Acceptance Criteria

- [ ] Go HTTP server serves frontend assets and exposes REST API for review data
- [ ] Document browser sidebar: lists discovered docs with title, status badge, open thread count, review status
- [ ] Reading view: rendered markdown with gutter markers at anchor positions; text selection triggers "Comment" affordance
- [ ] Thread panel: shows selected thread's comment history, reply input, status controls (resolve/reopen/won't fix)
- [ ] Status bar: repo/branch, document sync state, review sync state, pending local changes count with publish button
- [ ] Publish action: commits and pushes all pending changes via the data model layer

## Approach

Server in `internal/review/server.go` — standard `net/http` with JSON API endpoints. Frontend as embedded HTML/CSS/JS (no build step). Markdown rendered server-side to HTML via goldmark. Frontend receives pre-rendered HTML + annotation position metadata.

## Affected Modules

- `internal/review/server.go` (new) — HTTP server, API routes, static asset serving
- `internal/review/api.go` (new) — request/response types, handlers
- `internal/review/ui.go` (new) — embedded HTML/CSS/JS frontend
- `internal/cli/review.go` — calls server start + browser open

## Test Strategy

- API integration tests: exercise each endpoint against a temp repo with fixture data
- Verify document list includes correct metadata
- Verify thread creation via API produces correct JSON file
- Verify publish endpoint commits and pushes

## Out of Scope

- Real-time collaboration (websocket relay)
- Multi-repo dashboard
- Notifications
- Mobile-optimized layout
- Document editing within the review UI

## Notes

- Reference: docs/docs-review.md §10, §12
- Follow `present.go` pattern: serve on localhost, auto-open browser, print URL to stdout
