#!/bin/bash
set -e

# Sync templates from .claude/ (source of truth) to cmd/draft/templates/.claude/ (embed location)
# This script is used both in local development (via Makefile) and in CI/CD pipelines

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SOURCE_DIR="$PROJECT_ROOT/.claude"
DEST_DIR="$PROJECT_ROOT/cmd/draft/templates/.claude"

echo "Syncing templates from .claude/ to cmd/draft/templates/.claude/..."

# Clean destination directory
rm -rf "$DEST_DIR"

# Create destination directories
mkdir -p "$DEST_DIR/commands"
mkdir -p "$DEST_DIR/specs"

# Copy all command files
if [ -d "$SOURCE_DIR/commands" ]; then
    cp "$SOURCE_DIR/commands"/*.md "$DEST_DIR/commands/" 2>/dev/null || true
    echo "  ✓ Copied commands: $(ls -1 "$SOURCE_DIR/commands"/*.md 2>/dev/null | wc -l | tr -d ' ') files"
fi

# Copy only TEMPLATE.md from specs (exclude actual spec files)
if [ -f "$SOURCE_DIR/specs/TEMPLATE.md" ]; then
    cp "$SOURCE_DIR/specs/TEMPLATE.md" "$DEST_DIR/specs/"
    echo "  ✓ Copied TEMPLATE.md from specs"
fi

echo "Template sync complete!"
