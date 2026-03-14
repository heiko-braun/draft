#!/bin/bash
set -e

# Sync templates from .claude/, .cursor/, and specs/ (source of truth) to cmd/draft/templates/ (embed location)
# This script is used both in local development (via Makefile) and in CI/CD pipelines

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CLAUDE_SOURCE_DIR="$PROJECT_ROOT/.claude"
CLAUDE_DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/.claude"
CURSOR_SOURCE_DIR="$PROJECT_ROOT/.cursor"
CURSOR_DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/.cursor"
SPECS_SOURCE_DIR="$PROJECT_ROOT/specs"
SPECS_DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/specs"
PRINCIPLES_SOURCE_DIR="$PROJECT_ROOT/.principles"
PRINCIPLES_DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/.principles"

echo "Syncing templates from .claude/, .cursor/, .principles/, and specs/ to cmd/draft/templates/..."

# Clean destination directories
rm -rf "$CLAUDE_DEST_DIR"
rm -rf "$CURSOR_DEST_DIR"
rm -rf "$SPECS_DEST_DIR"
rm -rf "$PRINCIPLES_DEST_DIR"

# Create destination directories
mkdir -p "$CLAUDE_DEST_DIR/commands"
mkdir -p "$CLAUDE_DEST_DIR/agents"
mkdir -p "$CLAUDE_DEST_DIR/rules"
mkdir -p "$CURSOR_DEST_DIR"
mkdir -p "$SPECS_DEST_DIR"
mkdir -p "$PRINCIPLES_DEST_DIR"

# Copy all Claude command files
if [ -d "$CLAUDE_SOURCE_DIR/commands" ]; then
    cp "$CLAUDE_SOURCE_DIR/commands"/*.md "$CLAUDE_DEST_DIR/commands/" 2>/dev/null || true
    echo "  ✓ Copied Claude commands: $(ls -1 "$CLAUDE_SOURCE_DIR/commands"/*.md 2>/dev/null | wc -l | tr -d ' ') files"
fi

# Copy all Claude agent files
if [ -d "$CLAUDE_SOURCE_DIR/agents" ]; then
    cp "$CLAUDE_SOURCE_DIR/agents"/*.md "$CLAUDE_DEST_DIR/agents/" 2>/dev/null || true
    echo "  ✓ Copied Claude agents: $(ls -1 "$CLAUDE_SOURCE_DIR/agents"/*.md 2>/dev/null | wc -l | tr -d ' ') files"
fi

# Copy all Claude rule files
if [ -d "$CLAUDE_SOURCE_DIR/rules" ]; then
    cp "$CLAUDE_SOURCE_DIR/rules"/*.md "$CLAUDE_DEST_DIR/rules/" 2>/dev/null || true
    echo "  ✓ Copied Claude rules: $(ls -1 "$CLAUDE_SOURCE_DIR/rules"/*.md 2>/dev/null | wc -l | tr -d ' ') files"
fi

# Copy all Cursor skill files
if [ -d "$CURSOR_SOURCE_DIR" ]; then
    cp -r "$CURSOR_SOURCE_DIR"/* "$CURSOR_DEST_DIR/" 2>/dev/null || true
    skill_count=$(find "$CURSOR_SOURCE_DIR/skills" -name "SKILL.md" 2>/dev/null | wc -l | tr -d ' ')
    echo "  ✓ Copied Cursor skills: $skill_count files"
    rule_count=$(ls -1 "$CURSOR_SOURCE_DIR/rules"/*.md 2>/dev/null | wc -l | tr -d ' ')
    if [ "$rule_count" -gt 0 ]; then
        echo "  ✓ Copied Cursor rules: $rule_count files"
    fi
fi

# Copy only TEMPLATE.md from specs (exclude actual spec files)
if [ -f "$SPECS_SOURCE_DIR/TEMPLATE.md" ]; then
    cp "$SPECS_SOURCE_DIR/TEMPLATE.md" "$SPECS_DEST_DIR/"
    echo "  ✓ Copied TEMPLATE.md from specs"
fi

# Copy all principles files
if [ -d "$PRINCIPLES_SOURCE_DIR" ]; then
    cp "$PRINCIPLES_SOURCE_DIR"/*.md "$PRINCIPLES_DEST_DIR/" 2>/dev/null || true
    echo "  ✓ Copied principles: $(ls -1 "$PRINCIPLES_SOURCE_DIR"/*.md 2>/dev/null | wc -l | tr -d ' ') files"
fi

echo "Template sync complete!"
