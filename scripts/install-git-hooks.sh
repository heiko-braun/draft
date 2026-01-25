#!/bin/bash
# Install git hooks for draft development

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT_DIR="$(git rev-parse --git-dir)"

echo "Installing git hooks..."

# Install pre-commit hook
if [ -f "$GIT_DIR/hooks/pre-commit" ]; then
    echo "⚠️  Pre-commit hook already exists. Creating backup..."
    mv "$GIT_DIR/hooks/pre-commit" "$GIT_DIR/hooks/pre-commit.backup.$(date +%s)"
fi

cp "$SCRIPT_DIR/pre-commit" "$GIT_DIR/hooks/pre-commit"
chmod +x "$GIT_DIR/hooks/pre-commit"

echo "✅ Git hooks installed successfully!"
echo ""
echo "The pre-commit hook will now run automatically before each commit to check:"
echo "  • Code formatting (go fmt)"
echo "  • Dependency organization (go mod tidy)"
echo "  • Static analysis (go vet)"
echo ""
echo "To skip the hook temporarily, use: git commit --no-verify"
