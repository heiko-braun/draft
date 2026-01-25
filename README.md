# Claude Spec-Driven Development

A lightweight framework for spec-driven development with Claude Code. Define requirements clearly, get explicit confirmation, then implement with checkpoints.

## Why Spec-Driven?

When working with AI assistants on non-trivial features, jumping straight to code often leads to:
- Misunderstood requirements
- Wasted implementation effort
- Features that miss the mark

This framework adds a **specification phase** before implementation, ensuring alignment between what you want and what gets built.

## How It Works

```
/plan {feature description}
    │
    ▼
┌─────────────────────────┐
│   1. CLARIFY            │  Ask 3-5 questions (one at a time)
│      Questions          │  to understand requirements
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│   2. SPEC               │  Write lightweight spec to
│      Document           │  .claude/specs/{feature}.md
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│   3. CONFIRM            │  Get explicit user approval
│      Approval           │  before any implementation
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│   4. IMPLEMENT          │  Build in phases with
│      with Checkpoints   │  user checkpoints between each
└─────────────────────────┘
```

## Installation

Copy the `.claude/` directory to your project:

```bash
cp -r .claude/ /path/to/your/project/
```

Or clone and use as a template:

```bash
git clone https://github.com/your-org/claude-spec-driven.git
```

## Usage

### Start a Feature

```
/plan Add user authentication with OAuth support
```

Claude will:
1. Ask clarifying questions one at a time
2. Draft a spec based on your answers
3. Ask for confirmation before implementing

### Spec Format

Specs are stored in `.claude/specs/` with this structure:

```markdown
# Feature: {name}

## Goal
{What this accomplishes and why}

## Acceptance Criteria
- [ ] {Testable criterion 1}
- [ ] {Testable criterion 2}

## Approach
{2-3 sentences on implementation strategy}

## Out of Scope
- {Explicit exclusion 1}
- {Explicit exclusion 2}
```

### Implementation Phases

After spec approval, implementation proceeds in phases:

1. **Foundation** - Data models, types, schemas
2. **Core Logic** - Business logic, algorithms
3. **Integration** - Wire up components
4. **Polish** - Error handling, edge cases
5. **Verification** - Check acceptance criteria

After each phase, Claude pauses for your approval before continuing.

## Project Structure

```
.claude/
├── commands/
│   ├── plan.md                    # Entry point for /plan command
│   └── skills/
│       ├── spec/
│       │   ├── SKILL.MD           # Specification skill definition
│       │   └── TEMPLATE.md        # Spec file template
│       └── implement/
│           └── SKILL.md           # Implementation skill definition
└── specs/                         # Generated specs live here
    └── .gitkeep
```

## Skills Reference

### `/plan` Command

Entry point for spec-driven development. Triggers the spec skill, then implementation after approval.

### `spec` Skill

- Asks 3-5 clarifying questions (one at a time)
- Creates spec in `.claude/specs/{feature}.md`
- Requires explicit confirmation before proceeding

**When it activates:**
- Features involving multiple files
- Architectural decisions
- User-facing changes
- External integrations

**When it skips:**
- Simple bug fixes
- Single-line changes
- Documentation updates

### `implement` Skill

- Loads confirmed spec
- Breaks work into phases using TodoWrite
- Implements with checkpoint pauses
- Verifies against acceptance criteria
- Updates spec with completion status

## Benefits

- **Alignment**: Ensure you and Claude agree on what's being built
- **Control**: Pause points let you review, adjust, or stop
- **Documentation**: Specs serve as lightweight feature docs
- **Resumability**: Interrupted work can be continued from where you left off

## License

Apache 2.0
