# Feature: Migrate Specs to Top-Level Directory

## Goal

Move specification files from `.claude/specs/` to `/specs/` to make them first-class project documentation that is discoverable, tool-agnostic, and accessible to all team members regardless of their Claude Code usage.

## Acceptance Criteria

- [ ] All existing spec files migrated from `.claude/specs/` to `/specs/`
- [ ] `/spec` command creates new specs in `/specs/` instead of `.claude/specs/`
- [ ] `/implement` command reads specs from `/specs/` instead of `.claude/specs/`
- [ ] `/refine` command reads and updates specs in `/specs/` instead of `.claude/specs/`
- [ ] TEMPLATE.md moved to `/specs/TEMPLATE.md`
- [ ] `.claude/specs/` directory removed or left with just .gitkeep
- [ ] README.md updated to reflect new `/specs/` location
- [ ] CLI init command updated to create files in `/specs/` instead of `.claude/specs/`
- [ ] Embedded templates updated (cmd/draft/templates/.claude/specs/ references changed to point to /specs/)
- [ ] All documentation references to `.claude/specs/` updated to `/specs/`

## Approach

**Phase 1: Update command definitions**
- Modify `.claude/commands/spec.md` to write specs to `/specs/` instead of `.claude/specs/`
- Modify `.claude/commands/implement.md` to read from `/specs/` instead of `.claude/specs/`
- Modify `.claude/commands/refine.md` to read from `/specs/` instead of `.claude/specs/`

**Phase 2: Update CLI tool**
- Update `internal/cli/init.go` to create `/specs/` directory and copy TEMPLATE.md there
- Update conflict detection to check for files in `/specs/` instead of `.claude/specs/`
- Ensure the CLI creates `/specs/` at project root, not `.claude/specs/`

**Phase 3: Move existing specs**
- Create `/specs/` directory at project root
- Move all `.md` files from `.claude/specs/` to `/specs/` (except .gitkeep)
- Leave `.claude/specs/.gitkeep` or remove directory entirely

**Phase 4: Update documentation**
- Update README.md with new `/specs/` paths
- Update any other markdown files referencing `.claude/specs/`
- Update project structure diagram in README.md

**Phase 5: Sync templates**
- Run template sync to update embedded templates in `cmd/draft/templates/.claude/`
- Verify the build process handles the new structure correctly

## Out of Scope

- Supporting both locations simultaneously (clean cut-over)
- Automatic migration tool for existing users (manual migration via documentation is acceptable)
- Backward compatibility with old `.claude/specs/` location

## Notes

This change makes specs tool-agnostic and positions them as project documentation rather than Claude Code configuration. The `.claude/` directory will continue to house the workflow commands (spec.md, implement.md, refine.md) which are tool-specific, while the specs themselves become first-class project artifacts.
