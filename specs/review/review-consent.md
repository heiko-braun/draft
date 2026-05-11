---
title: Review Data Consent Gate
description: Require opt-in consent before draft review sends data to the default reviewd endpoint
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review Data Consent Gate

## Goal

Prevent `draft review` from transmitting data to the default reviewd service until the user explicitly opts in. Consent decision persists globally in a dotfile so the prompt appears only once.

## Acceptance Criteria

- [x] On first `draft review` invocation (no consent file exists), user sees a notice explaining data will be sent to the default endpoint, followed by a Y/n prompt
- [x] Accepting writes consent to `~/.config/draft/consent`
- [x] Declining prints a short message (how to grant later by editing the file) and exits non-zero
- [x] Subsequent invocations with consent granted proceed without prompting
- [x] Subsequent invocations with consent denied exit immediately without prompting
- [x] Consent check is skipped when `--server` or `REVIEWD_URL` is set (user-provided endpoint)

## Approach

Add a `consent` package under `internal/cli/consent/` that reads/writes `~/.config/draft/consent`. The file stores a single key (`review_data = true|false`). In `runReview`, after resolving the reviewd URL and before creating the client, call the consent check — but only when the resolved URL equals the hardcoded default. If consent is missing, prompt interactively; if denied, return an error.

## Affected Modules

- `internal/cli/consent/` (new) — consent read/write/prompt logic
- `internal/cli/review.go` — insert consent gate before client creation, only for default URL

## Test Strategy

- Unit tests for consent package: file missing → prompt needed, file present with true → ok, file present with false → denied
- Integration-level test: verify `runReview` returns error when consent denied and URL is default
- Verify no consent check triggers when `--server` is explicitly provided

## Out of Scope

- CLI command to manage consent (user edits dotfile directly)
- Consent for custom/self-hosted endpoints
- Migration or versioning of the consent file format
- Per-repo consent

## Notes

- XDG_CONFIG_HOME should be respected on Linux; macOS uses `~/.config/draft/` as well for simplicity
- The notice text should mention what data is sent: repo owner/name, document paths, review comments, user identity (GitHub token)
