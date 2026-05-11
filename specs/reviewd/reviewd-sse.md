---
title: Real-time Updates via SSE
description: Server-Sent Events for live thread and comment updates pushed to connected clients
status: proposed
author: Claude <noreply@anthropic.com>
---

# Feature: Real-time Updates via SSE

## Goal

Implement Server-Sent Events (SSE) so that connected clients receive live updates when threads or comments are created, updated, or deleted. This eliminates the need for polling and enables real-time collaboration between reviewers.

## Acceptance Criteria

- [ ] `GET /api/v1/repos/{owner}/{repo}/events` opens an SSE stream
- [ ] Events are scoped by repo — clients only receive events for the repo they're subscribed to
- [ ] Event types: `thread.created`, `thread.updated`, `thread.resolved`, `thread.reopened`, `thread.deleted`, `comment.created`
- [ ] Event payload is JSON containing the full entity (thread or comment) after the change
- [ ] Hub manages connected clients per repo, broadcasting events to all subscribers
- [ ] Clients authenticate via the same Bearer token (auth middleware applies)
- [ ] Heartbeat every 30 seconds to keep connections alive through proxies
- [ ] Clean disconnection handling (client drops, server shutdown)
- [ ] Handlers emit events after successful mutations (create/update/delete)

## Approach

Implement a `Hub` struct that manages SSE connections grouped by repo. Each connected client is a channel. When a mutation occurs (in any handler), the handler calls `hub.Broadcast(repoID, event)` after the database write succeeds. The hub iterates over registered clients for that repo and writes the SSE-formatted event.

The SSE endpoint uses standard HTTP streaming: set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, then write `data: {json}\n\n` for each event. Use `http.Flusher` to push data immediately.

## Affected Modules

- `internal/reviewd/hub.go` (new) — SSE hub, client registration, broadcasting
- `internal/reviewd/events.go` (new) — event types, payload structs
- `internal/reviewd/handlers.go` — emit events after successful mutations
- `internal/reviewd/routes.go` — register the `/events` SSE endpoint

## Event Format

```
event: thread.created
data: {"thread": {"id": "...", "document": "docs/spec.md", ...}}

event: comment.created
data: {"thread_id": "...", "comment": {"id": "...", "author": "...", "body": "..."}}

event: thread.resolved
data: {"thread": {"id": "...", "status": "resolved", "version": 5}}

:heartbeat
```

## Test Strategy

- Unit test: hub registers/unregisters clients correctly
- Unit test: broadcast delivers to all clients for a repo, not to other repos
- Unit test: slow client doesn't block other clients (buffered channels with drop)
- Integration test: connect SSE, create a thread via API, verify event received on stream
- Test: heartbeat arrives within 30 seconds on idle connection
- Test: client disconnect is handled without panics or goroutine leaks

## Out of Scope

- WebSocket support (SSE is sufficient for server-to-client push)
- Presence indicators (who's viewing what)
- Event persistence / replay on reconnect (clients use sync endpoint for catch-up)
- Horizontal scaling (multi-instance event distribution via Redis/NATS)

## Notes

- SSE is chosen over WebSocket because it's simpler (unidirectional), works through HTTP proxies, and auto-reconnects
- The `Last-Event-ID` header is NOT used for replay — clients that disconnect use the `/sync` endpoint with a timestamp to catch up
- Buffered channels (capacity 64) per client prevent slow consumers from blocking the hub; if a client's buffer is full, the event is dropped for that client
- The hub uses a mutex for the client registry, not channels, to keep the implementation simple
