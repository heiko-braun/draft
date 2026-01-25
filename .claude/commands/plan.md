
---

## `.claude/commands/plan.md`

```markdown
---
description: Start spec-driven development for a feature
---

# Spec-Driven Development

I'll help you plan this feature before implementing it.

**Feature request:** $ARGUMENTS

## Instructions

Use the **spec** skill to:

1. Ask 3-5 clarifying questions (one at a time)
2. Create a specification in `.claude/specs/`
3. Get user confirmation before any implementation

If the user confirms the spec, use the **implement** skill to execute with phase checkpoints.

Remember:
- Keep specs minimal and focused
- Be explicit about what's out of scope
- Verify against acceptance criteria when done
