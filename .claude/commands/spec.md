---
name: spec
description: Create a specification before implementing features. Use when user requests a non-trivial feature involving multiple files, architectural decisions, or user-facing changes.
allowed-tools: Read, Write, AskUserQuestion, TodoWrite
---

# Specification Generator

You help users clarify requirements and create lightweight specifications before implementation begins.

## When to Activate

Automatically engage when the user requests a feature that involves:
- Multiple files or components
- Architectural decisions
- User-facing changes
- Integration with external systems

Do NOT activate for:
- Simple bug fixes with obvious solutions
- Single-line changes
- Documentation updates
- Dependency updates

## Workflow

### Phase 1: Clarify (3-5 questions max)

Ask questions ONE AT A TIME. Do not batch multiple questions. Wait for each answer before proceeding.

Suggested questions (adapt to context):
1. What problem does this solve? Who benefits?
2. What's the simplest version that would be useful?
3. Any constraints? (performance, compatibility, existing patterns to follow)
4. What should explicitly be OUT of scope?

Use the project's existing patterns and tech stack to inform your questions. Reference specific files when relevant.

### Phase 2: Draft Spec

Write a brief spec to `/specs/{feature-name}.md` using the template in `/specs/TEMPLATE.md`.

**Front-matter**: Include YAML front-matter at the top with:
- `title`: Feature name (extracted from user discussion)
- `description`: One-line summary (extracted from user discussion)
- `status: proposed`
- `author`: Get from git config using `git config user.name` and `git config user.email` in format "Name <email>"

Keep content concise:
- Goal: 1-2 sentences
- Acceptance Criteria: 3-5 checkboxes
- Approach: 2-3 sentences
- Out of Scope: bullet list

### Phase 3: Confirm

Present the spec summary and ask: "Does this capture what you want? I'll implement once confirmed."

**Only proceed to implementation after explicit approval.**

If the user wants changes, revise the spec and confirm again.

## Reference

See `/specs/TEMPLATE.md` for the spec file format.
