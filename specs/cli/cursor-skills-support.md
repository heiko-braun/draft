---
title: Cursor Skills Support
description: Add Cursor-compatible skill format alongside existing Claude skills
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Cursor Skills Support

## Goal

Enable the draft CLI to initialize projects with Cursor-compatible skills (in `.cursor/skills/`) alongside the existing Claude slash command format, allowing users to choose their preferred AI coding agent while maintaining the same workflow automation capabilities.

## Acceptance Criteria

- [x] `draft init --agent cursor` creates only `.cursor/skills/` directory with skills in Cursor format
- [x] `draft init --agent claude` creates only `.claude/commands/` directory with Claude slash commands
- [x] `draft init` (no flag) creates both `.cursor/skills/` and `.claude/commands/` directories
- [x] All three existing skills (spec, implement, refine) are converted to Cursor SKILL.md format
- [x] Build commands/scripts sync Cursor skill files to embedded templates
- [x] `draft init` can load both skill formats from remote repositories
- [x] Both skill formats coexist in the repository without conflicts
- [x] Documentation updated to explain agent options

## Approach

Convert existing Claude slash commands to Cursor's skill standard format. Each skill will be a subdirectory under `.cursor/skills/` containing a `SKILL.md` file with YAML frontmatter (name, description) and markdown instructions. Update the `init` command to accept an `--agent` flag that determines which template structure to generate. Keep existing Claude templates unchanged in `cmd/draft/templates/.claude/` and create parallel Cursor templates in `cmd/draft/templates/.cursor/`. Update build process to ensure both `.claude/commands/` and `.cursor/skills/` are embedded into the binary and can be extracted during initialization from local or remote repositories.

## Out of Scope

- Runtime switching between agents
- Bi-directional sync of skills between formats
- Support for other AI coding tools beyond Cursor and Claude
- Modifying existing skill execution logic or workflow behavior
- Automatic migration or conversion tools

## Notes

Reference: [Cursor Skills Documentation](https://cursor.com/docs/context/skills)

The Cursor skill format uses:
- Directory structure: `.cursor/skills/{skill-name}/SKILL.md`
- YAML frontmatter with required fields: `name`, `description`
- Optional subdirectories: `scripts/`, `references/`, `assets/`
- Manual invocation via `/skill-name` or automatic context-based application

**Implementation Notes (2026-01-26)**:
- Created parallel `.cursor/` directory structure at project root alongside `.claude/`
- Updated `main.go` to embed both `.claude` and `.cursor` templates
- Modified `init.go` to accept `--agent` flag with validation and agent selection logic
- Updated `sync-templates.sh` to sync both Claude and Cursor templates from source directories
- Updated `Makefile` clean target to remove `.cursor/` build artifacts
- All three skills (spec, implement, refine) successfully converted to Cursor SKILL.md format
- Tested all three initialization modes: `--agent claude`, `--agent cursor`, and default (both)
