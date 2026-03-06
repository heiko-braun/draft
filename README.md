# Draft

**Shape how AI codes**

A lightweight framework for spec-driven development with Claude Code and other AI coding assistants.

## The Mission

AI assistants need better workflows. We're building them together.

Draft is a **community-driven framework** that teaches AI assistants to ask questions, create specs, and verify implementations before jumping to code.

**Help us define best practices for agentic coding.**

## The Workflow

Spec-driven development workflow encoded as reusable skills:

1. **`/spec`** - Clarify requirements, check scope, create spec
2. **`/implement`** - Assess blast radius, build as vertical slice
3. **`/verify-spec`** - Verify implementation matches spec
4. **`/refine`** - Refine existing spec at any point in time

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/heiko-braun/draft/main/install.sh | bash
```

Installs to `~/.local/bin` and configures PATH for bash and zsh.

### Other Installation Methods

**Using Go:**
```bash
go install github.com/heiko-braun/draft/cmd/draft@latest
```

**Manual Download:**

Download pre-built binaries from [releases](https://github.com/heiko-braun/draft/releases):

- **macOS (Intel)**: `draft-darwin-amd64`
- **macOS (ARM)**: `draft-darwin-arm64`
- **Linux**: `draft-linux-amd64`
- **Windows**: `draft-windows-amd64.exe`

See [install.sh](install.sh) for the installation script source.

## Quick Start

### 1. Initialize Your Project

```bash
draft init              # Claude Code & Cursor
draft init --agent claude   # Claude Code only
draft init --agent cursor   # Cursor only
```

### 2. Use the Skills

```
/spec Add user authentication
/implement authentication
/verify-spec authentication
/refine authentication
```

## Share & Discuss

View all specs in a web interface for screensharing and discussion with peers:

```bash
draft present
```

Opens a browser with rendered specs, table of contents, and metadata—perfect for team reviews and walkthroughs.

## Contribute

The workflows are defined in markdown files at `.claude/commands/` and `.cursor/skills/`. Test them, shape the details, share learnings.

[View on GitHub](https://github.com/heiko-braun/draft) →

## Spec Format

Specs are stored in `/specs/` with this structure:

```markdown
---
title: {Feature name}
description: {One-line summary}
status: proposed
author: {Name <email>}
---

# Feature: {name}

## Goal
{What this accomplishes and why}

## Acceptance Criteria
- [ ] {Testable criterion 1}
- [ ] {Testable criterion 2}
(3-5 criteria max — more suggests the scope is too large)

## Approach
{2-3 sentences on implementation strategy}

## Affected Modules
{List which modules/files change and where the boundary is}

## Test Strategy
{How criteria will be verified}

## Out of Scope
- {Explicit exclusion 1}
- {Explicit exclusion 2}
```

### Implementation as Vertical Slices

After spec approval, implementation proceeds as **one integrated piece** — types, logic, wiring, and tests together. Each spec represents a small, complete vertical slice delivered in one pass.

**Blast radius assessment**: Before coding, the assistant evaluates which modules will be touched and whether the change can be contained. If the blast radius is wider than expected, you'll be asked whether to proceed or restructure.

**Continuous testing**: Tests are written alongside implementation, not after. The full test suite runs before marking complete.

**Smart checkpoints**: The assistant pauses only when needed (unexpected blast radius, design trade-offs, test failures) rather than after arbitrary phases.

## Project Structure

```
.claude/                           # Claude Code workflow commands (SOURCE OF TRUTH)
├── commands/
│   ├── spec.md                    # Specification creation
│   ├── implement.md               # Implementation with checkpoints
│   └── refine.md                  # Refine existing specs

.cursor/                           # Cursor workflow skills (SOURCE OF TRUTH)
├── skills/
│   ├── spec/SKILL.md              # Specification creation
│   ├── implement/SKILL.md         # Implementation with checkpoints
│   └── refine/SKILL.md            # Refine existing specs

specs/                             # Project specifications (SOURCE OF TRUTH)
├── TEMPLATE.md                    # Spec template reference
└── {feature}.md                   # Generated specs

cmd/draft/templates/               # Build artifacts (git-ignored, auto-synced)
├── .claude/
├── .cursor/
└── specs/
```

**Note:** The `.claude/`, `.cursor/`, and `specs/` directories at the project root are the source of truth. Files in `cmd/draft/templates/` are automatically synced during builds and should never be edited directly.

## Commands Reference

### `/spec` Command

Creates a specification through a question-driven process.

**Process:**
1. Asks 3-5 clarifying questions (one at a time), including modularity considerations
2. Checks scope — if too large (>5 criteria, wide blast radius), suggests splitting into multiple specs
3. Creates spec in `/specs/{feature}.md` with YAML front-matter (title, description, status, author)
4. Presents spec for your review and confirmation

**Use when:**
- Features involving multiple files or architectural decisions
- User-facing changes or external integrations
- Non-trivial features that benefit from planning

**Skip when:**
- Simple bug fixes with obvious solutions
- Single-line changes or documentation updates

### `/implement` Command

Implements a feature from an existing specification as a single vertical slice.

**Process:**
1. Loads spec from `/specs/{feature}.md`
2. Assesses blast radius — which modules will be touched, can the change be contained?
3. Implements as one integrated piece (types, logic, wiring, tests together)
4. Tests continuously during implementation
5. Pauses only when needed (unexpected scope, design decisions, test failures)
6. Verifies against acceptance criteria when complete
7. Updates spec status from `proposed` to `implemented` and marks completed criteria

**Use when:**
- A spec has been created and confirmed with `/spec`
- Resuming interrupted implementation
- User explicitly says "implement {feature}"

### `/refine` Command

Updates an existing specification while preserving progress.

**Process:**
1. Loads existing spec from `/specs/`
2. Asks 2-3 focused refinement questions
3. Checks scope and modularity — flags if refinement expands blast radius
4. Updates spec in place (preserves front-matter and completed checkboxes)
5. Updates "Affected Modules" and "Test Strategy" if changes alter them
6. Shows diff summary and asks for confirmation
7. Documents changes with timestamp in Notes section

**Use when:**
- Spec needs updates based on feedback
- Requirements have changed slightly
- Implementation revealed new edge cases

**Create new spec instead when:**
- Scope is expanding significantly
- Core goals have completely changed


## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/heiko-braun/draft.git
cd draft

# Build (automatically syncs templates from .claude/)
make build

# Or build and install
make install
```

The build process automatically syncs templates from `.claude/` and `.cursor/` (source of truth) to `cmd/draft/templates/` (embed location) before building the binary.

### Template Source of Truth

- **Edit templates in**: `.claude/commands/*.md`, `.cursor/skills/*/SKILL.md`, and `specs/TEMPLATE.md`
- **Never edit**: `cmd/draft/templates/` (auto-generated build artifacts)
- **Manual sync**: `make sync-templates` (automatic when running `make build` or `make install`)

## License

Apache 2.0
