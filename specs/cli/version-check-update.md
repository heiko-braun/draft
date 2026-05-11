---
title: Version Check & Self-Update
description: Add --check and --update flags to `draft version` for checking and installing newer releases
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Version Check & Self-Update

## Goal
Let users check for newer releases and update the `draft` binary in-place, without leaving the CLI or manually downloading anything.

## Acceptance Criteria
- [ ] `draft version --check` queries GitHub Releases API, compares current version to latest, prints whether an update is available (with version numbers)
- [ ] `draft version --update` checks for a newer version and, if one exists, downloads and installs it by invoking the existing `install.sh` via a shell
- [ ] When already on the latest version, both flags print a "you're up to date" message and exit 0
- [ ] Errors (no network, API failure, script failure) produce a clear message and exit non-zero

## Approach
Add `--check` and `--update` boolean flags to the existing `version` subcommand. Version comparison uses semantic version parsing (strip `v` prefix, compare major.minor.patch). The update path shells out to `curl -fsSL <install.sh URL> | bash` with `DRAFT_VERSION` set to the latest tag — reusing the proven install script rather than reimplementing download/extract logic in Go.

## Affected Modules
- `internal/cli/version.go` — add flags, GitHub API call, semver compare, shell-out logic
- `cmd/draft/main.go` — no changes expected (version string already injected)

## Test Strategy
- Unit test: semver comparison function with cases like `0.5.5 < 0.6.0`, `1.0.0 = 1.0.0`, dirty/dev suffixes
- Unit test: parsing GitHub API JSON response to extract tag name
- Manual test: run `draft version --check` against real GitHub API, verify output
- Manual test: run `draft version --update` on a stale binary, verify it replaces itself with latest

## Out of Scope
- Automatic/background update checks on every CLI invocation
- Update channels (stable/beta), rollback, or pinning to a specific version
- Windows support for the shell-out path (install.sh is bash-only; Windows users use `go install`)
- Confirmation prompt before update — update flag is explicit intent

## Notes
- `install.sh` is hosted at `https://raw.githubusercontent.com/heiko-braun/draft/main/install.sh`
- The `version` variable in `main.go` may contain git-describe suffixes like `-28-gb603f04-dirty`; comparison should strip these to extract the base semver
