#!/bin/bash
set -e

# Sync templates from .claude/ and specs/ (source of truth) to cmd/draft/templates/ (embed location)
# This script is used both in local development (via Makefile) and in CI/CD pipelines

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CLAUDE_SOURCE_DIR="$PROJECT_ROOT/.claude"
CLAUDE_DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/.claude"
SPECS_SOURCE_DIR="$PROJECT_ROOT/specs"
SPECS_DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/specs"

echo "Syncing templates from .claude/ and specs/ to cmd/draft/templates/..."

# Clean destination directories
rm -rf "$CLAUDE_DEST_DIR"
rm -rf "$SPECS_DEST_DIR"

# Create destination directories
mkdir -p "$CLAUDE_DEST_DIR/commands"
mkdir -p "$SPECS_DEST_DIR"

# Copy all command files
if [ -d "$CLAUDE_SOURCE_DIR/commands" ]; then
    cp "$CLAUDE_SOURCE_DIR/commands"/*.md "$CLAUDE_DEST_DIR/commands/" 2>/dev/null || true
    echo "  ✓ Copied commands: $(ls -1 "$CLAUDE_SOURCE_DIR/commands"/*.md 2>/dev/null | wc -l | tr -d ' ') files"
fi

# Copy only TEMPLATE.md from specs (exclude actual spec files)
if [ -f "$SPECS_SOURCE_DIR/TEMPLATE.md" ]; then
    cp "$SPECS_SOURCE_DIR/TEMPLATE.md" "$SPECS_DEST_DIR/"
    echo "  ✓ Copied TEMPLATE.md from specs"
fi

echo "Template sync complete!"
