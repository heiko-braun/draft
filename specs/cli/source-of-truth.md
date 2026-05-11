---
title: Establish .claude/ as Source of Truth
description: Make .claude/ directory the single source of truth for templates embedded in draft CLI binary
status: implemented
author: Heiko braun <ike.braun@googlemail.com>
---

# Feature: Establish .claude/ as Source of Truth

## Goal

Make the project's `.claude/` directory the single source of truth for all templates and commands that get embedded in the draft CLI binary, eliminating duplicate files and ensuring consistency between development and distribution.

## Acceptance Criteria

- [x] `.claude/commands/*.md` files are copied to `cmd/draft/templates/.claude/commands/` during build
- [x] `.claude/specs/TEMPLATE.md` is copied to `cmd/draft/templates/.claude/specs/` during build
- [x] `cmd/draft/templates/.claude/` is git-ignored (copies are build artifacts only)
- [x] A shell script `scripts/sync-templates.sh` handles the copying logic
- [x] A `Makefile` provides targets for syncing templates and building the binary
- [x] The existing `//go:embed templates/.claude` directive continues to work without modification
- [x] GitHub Actions release workflow is updated to run the sync script before building
- [x] Documentation explains that `.claude/` is the source of truth and `cmd/draft/templates/` is build-only

## Approach

Create a shell script (`scripts/sync-templates.sh`) that copies files from `.claude/` to `cmd/draft/templates/.claude/`, excluding actual spec files (only the TEMPLATE.md). Add a Makefile with targets for syncing and building. Update `.gitignore` to exclude `cmd/draft/templates/.claude/`. Modify the GitHub Actions release workflow to run the sync script before building binaries. The existing embed directive in `cmd/draft/main.go:11` requires no changes.

## Out of Scope

- Automatically syncing templates during development (developers run `make sync-templates` manually or as part of `make build`)
- Hot-reloading of templates during development
- Embedding actual spec files from `.claude/specs/` (only TEMPLATE.md is embedded)
- Changing the embed location or the `//go:embed` directive structure

## Notes

- Current setup: `cmd/draft/templates/.claude/` contains duplicates of files from `.claude/`
- The sync script must preserve directory structure
- Both Makefile and script are needed: Makefile for local development, script for CI/CD pipeline
