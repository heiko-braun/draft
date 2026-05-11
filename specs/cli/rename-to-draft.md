---
title: Rename Project from claudespec to draft
description: Rebrand project to avoid trademark issues and make tool AI-assistant agnostic
status: implemented
author: Heiko braun <ike.braun@googlemail.com>
---

# Feature: Rename Project from claudespec to draft

## Goal

Rebrand the entire project from "claudespec" to "draft" to avoid Anthropic trademark issues and make the tool AI-assistant agnostic, supporting future expansion to assistants like Cursor while maintaining the specification-driven development methodology.

## Acceptance Criteria

- [x] Binary name changed from `claudespec` to `draft`
- [x] Go module renamed from `github.com/heiko-braun/claude-spec-driven` to `github.com/heiko-braun/draft`
- [x] All Go import paths updated throughout the codebase
- [x] CLI command directory renamed from `cmd/claudespec/` to `cmd/draft/`
- [x] Embedded templates path updated to `cmd/draft/templates/.claude/`
- [x] README updated with new name, tagline "Draft your specs before you code", and all command examples
- [x] CONTRIBUTING.md updated with new project name and paths
- [x] All documentation references changed from "claudespec" to "draft"
- [x] Makefile targets updated (binary name, build paths, install targets)
- [x] GoReleaser config updated (binary name, archive names, project references)
- [x] GitHub Actions workflows updated (any references to old names)
- [x] All spec files updated with new project name
- [x] Installation instructions use `draft` instead of `claudespec`
- [x] CLI help text and descriptions updated
- [x] Version command shows "draft version" instead of "claudespec version"
- [x] `.claude/` directory name remains unchanged (Claude Code compatibility)

## Approach

Perform a comprehensive find-and-replace rename across the entire project:

1. **Go Module and Imports:**
   - Update `go.mod` module path to `github.com/heiko-braun/draft`
   - Update all import statements in Go files
   - Run `go mod tidy` to verify

2. **Directory Structure:**
   - Rename `cmd/claudespec/` to `cmd/draft/`
   - Update embedded templates path references

3. **Binary and Build Configuration:**
   - Update Makefile: binary name, build paths, LDFLAGS
   - Update `.goreleaser.yaml`: binary name, archive naming, project metadata
   - Update GitHub Actions workflows if they reference the binary name

4. **Documentation:**
   - README.md: Project title, tagline, all code examples, installation URLs
   - CONTRIBUTING.md: All references to claudespec, module paths, build instructions
   - Update any inline documentation comments

5. **CLI Output:**
   - Update version command output
   - Update help text and command descriptions
   - Update any error messages that mention the tool name

6. **Verification:**
   - Build the binary: `make build`
   - Test `./bin/draft --version`
   - Test `./bin/draft init` in a test directory
   - Verify all help text shows "draft"
   - Verify embedded templates still work

## Out of Scope

- Renaming the GitHub repository (user will do manually)
- Renaming `.claude/` directory to `.draft/` (keeping for Claude Code compatibility)
- Creating new releases (will happen after merge)
- Updating existing GitHub releases (old releases keep old names)
- Migrating existing users (GitHub redirects handle this automatically)

## Notes

**Repository Rename:** The user will manually rename the GitHub repository from `claude-spec-driven` to `draft`. GitHub automatically sets up redirects, so old URLs continue to work.

**Claude Code Compatibility:** The `.claude/` directory name is intentionally kept unchanged to maintain compatibility with Claude Code and allow future support for other AI assistants (Cursor, etc.) while keeping assistant-specific configurations in their respective directories.

**Tagline:** "Draft your specs before you code" - emphasizes the planning-first approach with the metaphor of drafting (like architects draft blueprints).

**Search Strategy:** Use case-sensitive search for:
- `claudespec` (binary/command name)
- `claude-spec-driven` (repository/module name)
- `cmd/claudespec` (directory path)

Keep:
- `.claude/` directory references
- "Claude Code" mentions (the IDE)
- References to "Claude" the AI assistant where contextually appropriate
