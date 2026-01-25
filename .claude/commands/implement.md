---
name: implement
description: Implement features with phase checkpoints. Use after a spec exists in .claude/specs/ or when implementing a confirmed specification.
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, TodoWrite, AskUserQuestion
---

# Checkpoint-Aware Implementation

You implement features methodically with user checkpoints between phases.

## When to Activate

- After the spec skill has created and confirmed a specification
- When user says "implement" and a spec file exists
- When user explicitly references a spec file

## Workflow

### 1. Load Spec

Read the relevant `/specs/{feature}.md` file. If multiple specs exist and it's unclear which one, ask the user.

### 2. Create Task Breakdown

Use TodoWrite to break the spec into phases. Prefer this order when applicable:

1. **Foundation**: Data models, types, schemas
2. **Core Logic**: Business logic, algorithms, services
3. **Integration**: Connect components, wire up UI
4. **Polish**: Error handling, edge cases, validation
5. **Verification**: Test against acceptance criteria

Adapt phases to the specific feature - not all features need all phases.

### 3. Execute with Checkpoints

For each phase:

1. Announce what you're starting
2. Implement the phase
3. Summarize what was done (files changed, key decisions)
4. Ask: **"Phase complete. Ready for the next phase?"**
5. Only proceed on confirmation

This gives the user control to:
- Review changes before continuing
- Request adjustments
- Pause and resume later

### 4. Verify Against Spec

Before marking the feature complete:

1. Re-read the spec's acceptance criteria
2. Check each criterion explicitly
3. Report: "✓ Criterion met" or "⚠ Needs attention: {issue}"

Only mark complete when all criteria pass.

### 5. Update Spec

After successful implementation:
- Check off completed acceptance criteria in the spec file
- Add any notes about implementation decisions

## Checkpoint Behavior

**Do checkpoint** after:
- Completing a logical phase
- Making architectural decisions
- Any change that affects multiple files

**Skip checkpoint** for:
- Minor follow-up fixes within a phase
- Formatting/cleanup
- When user explicitly says "continue without asking"

## Recovery

If implementation is interrupted:
- TodoWrite preserves progress
- Spec file shows which criteria are done
- User can say "continue implementing {feature}" to resume
