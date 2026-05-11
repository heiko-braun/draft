---
title: Reviewd Admin UI
description: Simple admin dashboard showing summary stats (users, repos, comments)
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Reviewd Admin UI

## Goal

Provide a read-only admin dashboard at `/admin` on the reviewd server showing summary statistics (participants, repos, comments). Access restricted to users whose GitHub-verified email matches a configured admin email list.

## Acceptance Criteria

- [x] `ADMIN_EMAILS` env var (comma-separated) configures who can access `/admin`
- [x] `GET /admin` returns an HTML page with counts: total participants, total repos, total comments
- [x] Unauthenticated requests or non-admin users get 403
- [x] Page auto-refreshes or is static (no JS framework needed)

## Approach

Add `ADMIN_EMAILS` to config. Add an `AdminOnly` middleware that checks `AuthContext.Email` against the allowed list. Create a single handler serving an inline HTML page (same pattern as `internal/review/ui.go`) with stats queried from Postgres (`COUNT(*)` on `participants`, `repos`, `comments` tables). Register `GET /admin` behind auth + admin middleware in the server mux.

## Affected Modules

- `internal/reviewd/config.go` — parse `ADMIN_EMAILS` env var into `[]string`
- `internal/reviewd/auth.go` — add `AdminOnly()` middleware checking email against admin list
- `internal/reviewd/admin.go` (new) — handler + inline HTML template for the dashboard
- `internal/reviewd/store.go` — add `AdminStats()` query returning counts
- `internal/reviewd/server.go` — register `/admin` route

## Test Strategy

- Unit test: `AdminOnly` middleware returns 403 for non-admin email, 200 for admin email
- Unit test: `AdminStats()` returns correct counts against test DB
- Integration: `GET /admin` with valid admin token returns HTML containing expected count values
- Integration: `GET /admin` with non-admin token returns 403

## Out of Scope

- CRUD operations (no editing/deleting from admin UI)
- Pagination or drill-down into individual records
- Admin email management at runtime (env-var only)
- Styling beyond basic readability
