---
title: Spec Front-Matter
description: Add YAML front-matter to spec files with metadata (title, description, status, author) to support tooling and lifecycle tracking
status: implemented
author: Heiko braun <ike.braun@googlemail.com>
---

# Feature: Spec Front-Matter

## Goal

Add YAML front-matter to all spec files to enable better tooling (presentation view, dashboards) and lifecycle tracking. Front-matter will include title, description, status, and author fields that are automatically managed by `/spec`, `/refine`, and `/implement` commands.

## Acceptance Criteria

- [x] All new specs created by `/spec` include front-matter with `title`, `description`, `status: proposed`, and `author` (from git config)
- [x] `/refine` preserves existing front-matter and keeps `status: proposed`
- [x] `/implement` updates status to `implemented` when complete
- [x] TEMPLATE.md is updated to include front-matter example
- [x] Existing slash command files (spec.md, refine.md, implement.md) are updated with front-matter handling instructions

## Approach

Update the three slash command files to handle front-matter. In `/spec`: extract title/description from user clarifications, set `status: proposed`, read git user from `git config user.name` and `git config user.email`. In `/refine`: preserve all front-matter fields, maintain `status: proposed`. In `/implement`: update status to `implemented` at completion. Update TEMPLATE.md to show front-matter format with all four fields.

## Out of Scope

- Automatic status detection (user-driven only via commands)
- Front-matter validation or schema enforcement
- Migration of existing specs (they'll be updated organically as they're refined)
- Additional front-matter fields (tags, dates, etc.)

## Notes

Status values: `proposed` (new or refined), `implemented` (done).
