---
title: GitHub OAuth Auth Middleware
description: HTTP middleware that verifies GitHub OAuth tokens and repo access permissions
status: proposed
author: Claude <noreply@anthropic.com>
---

# Feature: GitHub OAuth Auth Middleware

## Goal

Implement HTTP middleware that authenticates requests using GitHub OAuth tokens and authorizes access based on the user's repo permissions. Identity is derived entirely from GitHub — no separate user accounts. The middleware extracts the token from the `Authorization` header, verifies it with the GitHub API, and injects user identity and permission level into the request context.

## Acceptance Criteria

- [ ] Middleware extracts `Bearer <token>` from `Authorization` header
- [ ] Calls GitHub API (`GET /user`) to verify token and get user identity
- [ ] Caches token → user mapping with TTL (5 minutes) to avoid hammering GitHub API
- [ ] For repo-scoped endpoints, checks repo access via `GET /repos/{owner}/{repo}`
- [ ] Maps GitHub repo permission to access level: `read` (pull), `write` (push/triage), `admin`
- [ ] Injects `AuthContext` (user ID, login, email, access level) into `context.Context`
- [ ] Returns 401 for missing/invalid tokens
- [ ] Returns 403 for insufficient repo permissions
- [ ] Health/readiness endpoints (`/healthz`, `/readyz`) bypass auth
- [ ] Participant auto-creation: on first authenticated request, ensure participant exists in DB

## Approach

The middleware is a standard `func(http.Handler) http.Handler` wrapper. It runs before all API handlers. Token verification results are cached in a sync.Map with expiry timestamps to minimize GitHub API calls.

The GitHub API client is a thin wrapper using `net/http` — no external GitHub SDK dependency. It calls two endpoints: `/user` for identity and `/repos/{owner}/{repo}` for permission checks.

Repo-scoped routes include the repo identifier in the URL path (`/api/v1/repos/{owner}/{repo}/...`), which the middleware uses for the permission check.

## Affected Modules

- `internal/reviewd/auth.go` (new) — middleware, GitHub API client, token cache
- `internal/reviewd/context.go` (new) — `AuthContext` type, context helpers (`UserFromContext`)
- `internal/reviewd/server.go` — wire middleware into route chain, skip for health endpoints

## Test Strategy

- Unit test: token extraction from Authorization header (valid, missing, malformed)
- Unit test: context injection and retrieval of AuthContext
- Unit test: cache hit avoids second GitHub API call
- Unit test: cache expiry triggers re-verification
- Integration test: mock GitHub API server, verify full auth flow (valid token → 200, invalid → 401, no repo access → 403)
- Test: health endpoints return 200 without auth header

## Out of Scope

- OAuth login flow (token exchange, redirect) — clients obtain tokens externally
- GitHub App installation model (Phase 3 from SaaS proposal)
- Rate limit handling for GitHub API calls
- Token refresh / re-authentication flows

## Notes

- The middleware is intentionally simple — no OAuth flow, just token verification
- Clients are expected to already have a GitHub personal access token or OAuth token
- The `AuthContext.ParticipantID` is derived from the GitHub user's email, matching the existing participant ID scheme in `internal/review/store.go`
- Permission mapping: GitHub's `pull` → read, `push`/`triage` → write, `admin` → admin
