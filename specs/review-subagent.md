---
title: Review Subagent
description: Verify implemented code changes match their specification
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: Review Subagent

## Goal

Add a review subagent that verifies whether implemented code changes match the specification they reference. The subagent reports mismatches between spec and implementation, and offers to fix issues when found.

## Acceptance Criteria

- [x] Slash command `/verify-spec` invokes the review subagent
- [x] Subagent reads the referenced spec file and identifies affected code changes
- [x] Subagent verifies each acceptance criterion in the spec is addressed by the implementation
- [x] Subagent checks that the affected modules listed in spec were actually modified
- [x] Subagent runs test suite and reports pass/fail status
- [x] Subagent generates a review report with findings (spec compliance, test results)
- [x] When mismatches are found, subagent asks user if it should fix them
- [x] Subagent NEVER modifies the spec file (spec is source of truth)

## Approach

Create a new slash command file `.claude/commands/review.md` following the same pattern as `implement.md` and `spec.md`. The command will:

1. Identify which spec to review (auto-detect from git history or ask user)
2. Load the spec file and parse acceptance criteria and affected modules
3. Use git diff to analyze code changes since spec creation
4. Verify each acceptance criterion against actual code changes
5. Run test suite and capture results
6. Generate review report with compliance status
7. Offer to fix identified issues (but never modify spec itself)

The review logic will be implemented as instructions in the command file, leveraging existing Claude Code tools (Read, Grep, Bash, TodoWrite).

## Affected Modules

- `.claude/commands/verify-spec.md` — new slash command file defining the review workflow
- No changes to existing command files or spec template

The change is fully contained within a new command file. No shared code is modified.

## Test Strategy

- Manually test `/verify-spec` command on an existing implemented spec
- Verify it correctly identifies spec file from git history
- Verify it parses acceptance criteria correctly
- Verify it detects code changes in affected modules
- Verify it runs tests and reports results
- Verify it detects mismatches when code doesn't match spec
- Verify it never attempts to modify spec files
- Test both scenarios: clean implementation (all criteria met) and incomplete implementation (criteria missing)

## Out of Scope

- Automatic triggering after `/implement` completes (manual invocation only)
- Code style/linting checks (focus only on spec compliance)
- Applying fixes without user confirmation
- Support for Cursor editor integration (Claude Code only for now)
- Automated commit of review findings

## Notes

Review workflow inspired by the Conductor project's review command but simplified to focus on spec-to-code verification. The subagent treats the spec as the immutable source of truth.
