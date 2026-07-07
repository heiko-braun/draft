---
title: CLI Self-Update
description: draft update command plus background release check with 24h cache
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: self-update

## Goal

Let users upgrade `draft` to the latest GitHub release without leaving the CLI, and notify them passively when a newer release exists.

## Acceptance Criteria

- [x] `draft update` re-runs official install script (`curl -fsSL .../install.sh | bash`), replacing current binary
- [x] `draft update --check` reports whether a newer version is available without installing
- [x] On every command, CLI checks GitHub Releases API for latest tag; result cached 24h in user cache dir
- [x] When cached latest > current version and stderr is a TTY, print one-liner at end: `Update available: vX → vY. Run 'draft update'`
- [x] Check skipped when stderr not a TTY, when version is `dev`, when `DRAFT_NO_UPDATE_CHECK` set, or when cache is fresh
- [x] Network/API failures are silent (no error output, no blocked commands)

## Approach

New `internal/selfupdate` package exposes two entry points: `CheckAsync(currentVersion)` fired from CLI root before command dispatch (writes notice on exit), and `Run(currentVersion)` invoked by a new `draft update` cobra command that shells out to `bash -c 'curl -fsSL <install.sh URL> | bash'`. Version comparison via `golang.org/x/mod/semver`. Cache file at `os.UserCacheDir()/draft/latest.json` with `{tag, checked_at}`.

## Affected Modules

- `internal/selfupdate/` — new package: GitHub API fetch, cache read/write, semver compare, install-script runner. Contains all self-update logic behind a small API.
- `internal/cli/` — register new `update` command; call `selfupdate.CheckAsync` from root pre-run and flush notice on post-run
- `cmd/draft/main.go` — pass `version` into selfupdate (already available)
- `go.mod` — add `golang.org/x/mod` dep

Boundary: everything except command registration lives in `internal/selfupdate`; CLI layer only wires it.

## Test Strategy

- Unit tests in `internal/selfupdate`: semver compare, cache freshness logic, JSON parse of GitHub release payload (fixture), TTY-skip decision
- Manual: build with `-X main.version=v0.0.1`, run against real GitHub API, verify notice appears; re-run within 24h, verify no API call (inspect cache mtime); pipe output, verify no notice
- Manual: run `draft update` in disposable dir, verify install script runs and binary is replaced

## Out of Scope

- Automatic install without user confirmation
- Rollback to previous version
- Pre-release / channel selection (stable only)
- Windows support for `draft update` (install.sh is bash) — command prints manual instructions on Windows
- Detecting `go install` origin — update always uses install.sh
- Signature / checksum verification beyond what install.sh already does

## Notes

Install script URL hardcoded: `https://raw.githubusercontent.com/heiko-braun/draft/main/install.sh`. GitHub API: `https://api.github.com/repos/heiko-braun/draft/releases/latest` (unauthenticated, 60 req/hr/IP — fine given 24h cache).

## Implementation notes

- Kept code in `internal/cli/` rather than a new `internal/selfupdate` package: `version.go` already had `runUpdate`/`runCheck`/`fetchLatestVersion`/`parseSemver`. Extracting would have duplicated it.
- New files: `internal/cli/update.go` (top-level `draft update` cobra command with `--check`), `internal/cli/notice.go` (background check + 24h cache), `internal/cli/notice_test.go`.
- Notice prints to **stderr** (not stdout) so it never pollutes piped output; TTY check is on stderr.
- Additional escape hatch: `DRAFT_NO_UPDATE_CHECK` env var disables the background check.
- `draft update` command uses the existing `runUpdate()`. `draft version --update` is preserved for backward compatibility.
