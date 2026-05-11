# Threat Model: reviewd Authentication & Authorization

**Date:** 2026-05-11
**Scope:** GitHub repo access control on all reviewd API routes
**Files analyzed:**
- `internal/reviewd/auth.go`
- `internal/reviewd/context.go`
- `internal/reviewd/server.go`
- `internal/reviewd/handlers.go`
- `internal/reviewd/config.go`
- `cmd/reviewd/main.go`

---

## 1. Executive Summary

The reviewd service delegates authentication and authorization entirely to GitHub's API. Bearer tokens (typically GitHub PATs or `gh` CLI tokens) are forwarded to GitHub's `/user` and `/repos/{owner}/{repo}` endpoints to verify identity and repository permissions. Results are cached in-memory for 5 minutes.

**Overall Risk Level: MEDIUM-HIGH**

The design is fundamentally sound -- delegating authz to GitHub means the service inherits GitHub's permission model. However, several implementation-level issues create exploitable gaps:

- **Token stored as plaintext cache key** enables memory-dump token theft
- **No cache size limits** create a denial-of-service vector
- **Cross-repository resource access** is not validated at the handler layer
- **No token scope validation** means over-privileged tokens are accepted silently
- **No rate limiting** on authentication failures allows credential brute-forcing

---

## 2. Data Flow Analysis

### Critical Assets

| Asset | Location | Sensitivity |
|-------|----------|-------------|
| GitHub Bearer tokens | In-flight (HTTP header), in-memory cache keys | HIGH - grants access to user's GitHub resources |
| User identity (AuthContext) | Request context, in-memory cache | MEDIUM - PII (email, login) |
| Review data (threads, comments) | PostgreSQL database | MEDIUM - potentially confidential code review content |
| Database connection string | Environment variable | HIGH - full DB access |
| GitHub OAuth secrets | Environment variables (unused) | HIGH - could enable OAuth impersonation |

### Data Flow Diagram (Textual)

```
CLI (gh auth token / GITHUB_TOKEN)
  |
  | Bearer token in Authorization header
  v
reviewd (auth middleware)
  |
  |--- token cache hit? ---> proceed to handler
  |
  |--- cache miss ---> GitHub /user API (verify identity)
  |                         |
  |                         v
  |                    cache token -> AuthContext
  |
  |--- RequireRepoAccess ---> repo access cache hit? ---> proceed
  |                      |
  |                      |--- cache miss ---> GitHub /repos/{o}/{r} API
  |                                                |
  |                                                v
  |                                           cache access level
  v
Handler (reads/writes PostgreSQL)
  |
  v
SSE Hub (broadcasts events to connected clients)
```

### Potential Data Leakage Paths

1. **Token in memory** -- cache stores raw tokens as map keys; memory dump or core dump exposes them
2. **Debug logging** -- logger receives path/user info; if log level is debug, auth metadata is written to stdout
3. **Error messages** -- internal errors from the store are returned verbatim to clients (e.g., `err.Error()`)
4. **SSE broadcast** -- events are sent to all subscribers of a repo channel without per-connection re-auth
5. **Health endpoint** -- `/readyz` leaks database error messages to unauthenticated callers

---

## 3. Threat Analysis (STRIDE)

### 3.1 Spoofing

| ID | Threat | Impact | Likelihood | Details |
|----|--------|--------|------------|---------|
| S1 | Stolen/leaked GitHub token reuse | HIGH | HIGH | Tokens are not audience-restricted. Any valid GitHub PAT with repo scope works, regardless of intended audience. An attacker who obtains a token from CI logs, env leaks, or clipboard can impersonate that user for 5 minutes (cache TTL) even after revocation. |
| S2 | Token reuse after GitHub revocation | MEDIUM | MEDIUM | Once cached, a revoked token remains valid in reviewd for up to 5 minutes. No mechanism exists to invalidate the cache entry. |
| S3 | Service impersonation (no TLS enforcement) | HIGH | LOW | The server listens on plain HTTP. If deployed without a TLS-terminating proxy, tokens transit in cleartext. Railway likely provides TLS, but nothing in the code enforces or validates this. |

### 3.2 Tampering

| ID | Threat | Impact | Likelihood | Details |
|----|--------|--------|------------|---------|
| T1 | Cross-repo thread manipulation | HIGH | MEDIUM | `handleGetThread`, `handleDeleteThread`, and `handleGetReview` fetch resources by ID without verifying the resource belongs to the `{owner}/{repo}` in the URL. An attacker with write access to repo A can delete a thread belonging to repo B by guessing/enumerating the thread ID, because the repo access check only validates the URL path repo. |
| T2 | Mutation injection via publish endpoint | MEDIUM | MEDIUM | The `/publish` endpoint accepts arbitrary mutations. While write access is checked for the URL repo, an `upsert_thread` mutation could reference a `ReviewID` belonging to a different repo. No cross-validation occurs. |
| T3 | Participant impersonation in publish | MEDIUM | LOW | The `add_comment` operation in publish uses the authenticated user's ParticipantID. However, `upsert_thread` takes a full `review.Thread` struct from the client, potentially allowing thread ownership/metadata manipulation. |

### 3.3 Repudiation

| ID | Threat | Impact | Likelihood | Details |
|----|--------|--------|------------|---------|
| R1 | No audit trail for destructive operations | MEDIUM | HIGH | Deletes, resolves, and status changes are not logged with actor identity at INFO level. Only DEBUG-level logging captures the user context. In production (INFO level), there is no attribution of who performed a destructive action. |
| R2 | Participant ID collision enables plausible deniability | LOW | LOW | ParticipantID is a 12-hex-char (48-bit) hash of email. While collision probability is low for typical user counts, it is non-cryptographic in the sense that a determined attacker could generate a colliding email to attribute actions to someone else. |

### 3.4 Information Disclosure

| ID | Threat | Impact | Likelihood | Details |
|----|--------|--------|------------|---------|
| I1 | Internal error messages exposed to clients | MEDIUM | HIGH | Multiple handlers return `err.Error()` directly (e.g., database errors, store errors). These may reveal table names, SQL syntax, connection details, or internal state. |
| I2 | Readyz endpoint leaks database error details | LOW | HIGH | `/readyz` returns the database ping error to unauthenticated callers, potentially revealing the database host, port, or driver errors. |
| I3 | Token stored as plaintext in cache | HIGH | LOW | The `tokenCache` uses the raw token string as the map key. A heap dump, core dump, or memory-inspection side-channel exposes all recently-used tokens. |
| I4 | Timing side-channel on cache lookup | LOW | LOW | Cache hits return faster than misses. An attacker can determine whether a token was recently used by observing response latency, though exploitation value is limited. |
| I5 | GitHub token forwarded with full scope | MEDIUM | MEDIUM | The user's token (which may have broad scopes: `repo`, `admin:org`, etc.) is forwarded to GitHub APIs. If the reviewd server were compromised, all cached tokens grant far more access than reviewd needs. |

### 3.5 Denial of Service

| ID | Threat | Impact | Likelihood | Details |
|----|--------|--------|------------|---------|
| D1 | Unbounded cache growth (memory exhaustion) | HIGH | MEDIUM | Both `tokenCache` and `repoAccessCache` use `sync.Map` with no eviction policy beyond TTL checks on read. An attacker can send millions of unique invalid tokens, each causing a GitHub API call that fails, but the entries (or at least map keys) accumulate. Valid-but-expired entries are only cleaned on next access, not proactively. |
| D2 | GitHub API rate-limit exhaustion | HIGH | MEDIUM | Each unique token causes a GitHub API call on cache miss. An attacker flooding the service with distinct tokens will exhaust GitHub's rate limit (60/hr for unauthenticated, 5000/hr for authenticated), effectively blocking legitimate auth. |
| D3 | SSE connection exhaustion | MEDIUM | MEDIUM | The `/events` endpoint establishes long-lived SSE connections. No per-user or per-IP connection limit is enforced. An attacker can exhaust server connections/goroutines. |
| D4 | No request body size limit | MEDIUM | LOW | `decodeJSON` does not limit the request body size. A large publish request with thousands of mutations or enormous comment bodies could exhaust memory. |

### 3.6 Elevation of Privilege

| ID | Threat | Impact | Likelihood | Details |
|----|--------|--------|------------|---------|
| E1 | Cross-repo resource access via ID guessing | HIGH | MEDIUM | As described in T1: the authorization check validates access to the repo in the URL, but handlers fetch resources by global ID without verifying repo ownership. A user with write access to any repo can operate on threads/reviews in other repos. **This is the most critical finding.** |
| E2 | Sync endpoint uses read-level access for POST | LOW | LOW | `POST /api/v1/repos/{owner}/{repo}/sync` only requires `AccessRead`. While the operation is semantically read-only (bulk pull), using POST for reads is unconventional and could confuse WAF/proxy rules that block POST without write tokens. |
| E3 | No token scope validation | MEDIUM | MEDIUM | Any valid GitHub token is accepted regardless of its OAuth scopes. A fine-grained PAT with only `metadata:read` scope should not be able to read review content, but the service only checks repo permissions, not token scopes. |

---

## 4. Security Controls & Mitigations

### Priority 1 (Critical -- address before next release)

| Finding | Mitigation | Effort |
|---------|-----------|--------|
| E1/T1: Cross-repo resource access | Add repo-ownership validation in all handlers that fetch by ID. Before returning or mutating a thread/review, verify `thread.RepoID == repo.ID` from the URL-derived repo. Return 404 (not 403) to avoid confirming existence. | Medium |
| I1: Internal error leakage | Replace `err.Error()` in HTTP responses with generic messages. Log the real error server-side at WARN/ERROR level. | Low |
| D1: Unbounded cache | Replace `sync.Map` with a bounded LRU cache (e.g., `hashicorp/golang-lru` or a custom structure with max size). Add a background goroutine to sweep expired entries. | Medium |

### Priority 2 (High -- address within current sprint)

| Finding | Mitigation | Effort |
|---------|-----------|--------|
| S1/S2: Token reuse after revocation | Reduce cache TTL to 1-2 minutes for token verification. Consider a revocation webhook or periodic re-validation for long-lived SSE connections. | Low |
| D2: GitHub API rate-limit exhaustion | Add rate limiting on auth failures per source IP (e.g., 10 failures/minute). Return 429 early for IPs exceeding threshold. | Medium |
| I3: Plaintext token in cache | Use a SHA-256 hash of the token as the cache key instead of the raw token. The token is still in memory briefly during the request, but not persisted in the map. | Low |
| I2: Readyz error leakage | Return only `{"status": "unavailable"}` without the error message to unauthenticated callers. Log the error server-side. | Low |
| D3: SSE connection limits | Enforce max connections per authenticated user (e.g., 5) and total connection cap. Drop oldest connection on overflow. | Medium |

### Priority 3 (Medium -- address in next iteration)

| Finding | Mitigation | Effort |
|---------|-----------|--------|
| R1: Missing audit logging | Log all state-changing operations (create, update, delete) at INFO level with actor identity (GitHubLogin), resource ID, and repo. | Low |
| D4: No body size limit | Wrap request bodies with `http.MaxBytesReader` (e.g., 1 MB for publish, 64 KB for comments). | Low |
| S3: No TLS enforcement | Add a startup check or configuration flag requiring `HTTPS_ONLY=true` that rejects non-TLS connections (or document that a TLS proxy is required for production). | Low |
| E3: Token scope validation | Call GitHub's `/applications/{client_id}/token` endpoint or inspect `X-OAuth-Scopes` response header from `/user` to verify the token has appropriate scopes. | Medium |
| T2/T3: Publish mutation validation | Validate that all resources referenced in publish mutations belong to the target repo. Ignore client-supplied metadata fields that should be server-controlled. | Medium |

### Priority 4 (Low -- defense-in-depth)

| Finding | Mitigation | Effort |
|---------|-----------|--------|
| R2: ParticipantID collision | Increase hash length from 48-bit to at least 128-bit, or use GitHub numeric ID directly as the participant identifier. | Low |
| I4: Timing side-channel | Not practically exploitable in this context. No action needed. | -- |
| E2: POST for read operation | Consider changing `/sync` to GET with query parameters, or document the design decision. | Low |

---

## 5. Architecture Observations

### Positive Security Properties

- **No credential storage** -- the service never stores GitHub tokens persistently; they exist only in volatile cache
- **Principle of least authority at the route level** -- read vs. write access is distinguished per endpoint
- **Short cache TTL** -- 5-minute window limits blast radius of token compromise
- **Graceful shutdown** -- prevents connection-state corruption
- **Method-specific routing** -- Go 1.22 method patterns prevent verb confusion attacks

### Residual Risks Accepted

- **Full delegation to GitHub** -- if GitHub's permission model has bugs, reviewd inherits them
- **Single-instance cache** -- if scaled horizontally, each instance maintains independent cache state (no consistency guarantee, but also no shared attack surface)
- **No CSRF protection** -- acceptable because the API uses Bearer tokens (not cookies)

---

## 6. Recommended Security Testing

1. **Penetration test the cross-repo access flaw (E1/T1)** -- create threads in repo A, attempt to read/modify/delete from the API endpoint of repo B using a valid token for B
2. **Fuzz the publish endpoint** -- send malformed, oversized, and cross-referencing mutations
3. **Cache exhaustion test** -- send 100k unique tokens and monitor memory growth
4. **Token revocation test** -- revoke a token on GitHub and verify it stops working within acceptable window
5. **SSE connection saturation** -- open maximum SSE connections and verify the server remains responsive to new HTTP requests
