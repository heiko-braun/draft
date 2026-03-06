#!/bin/bash
set -e

# Draft installation script
# Installs draft to ~/.local/bin and configures PATH for bash and zsh

VERSION=${DRAFT_VERSION:-latest}
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="draft"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    linux*)
        OS="linux"
        ;;
    darwin*)
        OS="darwin"
        ;;
    msys*|mingw*|cygwin*)
        OS="windows"
        BINARY_NAME="draft.exe"
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Get latest version tag if using 'latest'
if [ "$VERSION" = "latest" ]; then
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -fsSL https://api.github.com/repos/heiko-braun/draft/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- https://api.github.com/repos/heiko-braun/draft/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        echo "Error: curl or wget is required to download draft"
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
fi

# Construct download URL
ARCHIVE_NAME="draft_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/heiko-braun/draft/releases/download/${VERSION}/${ARCHIVE_NAME}"

echo "Installing Draft ${VERSION}..."
echo "  OS: $OS"
echo "  Architecture: $ARCH"
echo "  Install directory: $INSTALL_DIR"
echo ""

# Create install directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Download and extract archive
echo "Downloading draft..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/$ARCHIVE_NAME"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$DOWNLOAD_URL" -O "$TEMP_DIR/$ARCHIVE_NAME"
else
    echo "Error: curl or wget is required to download draft"
    exit 1
fi

# Extract binary
echo "Extracting draft..."
tar -xzf "$TEMP_DIR/$ARCHIVE_NAME" -C "$TEMP_DIR"

# Move binary to install directory
mv "$TEMP_DIR/draft" "$INSTALL_DIR/$BINARY_NAME"

# Make binary executable
chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo "✓ Draft installed to $INSTALL_DIR/$BINARY_NAME"
echo ""

# Configure PATH for bash and zsh
configure_path() {
    local shell_rc=$1
    local shell_name=$2

    if [ -f "$shell_rc" ]; then
        # Check if PATH already includes ~/.local/bin
        if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' "$shell_rc" && \
           ! grep -q 'export PATH=$HOME/.local/bin:$PATH' "$shell_rc"; then
            echo "" >> "$shell_rc"
            echo "# Added by Draft installer" >> "$shell_rc"
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$shell_rc"
            echo "✓ Updated $shell_name configuration ($shell_rc)"
            return 0
        else
            echo "✓ $shell_name configuration already includes ~/.local/bin"
            return 1
        fi
    fi
    return 2
}

PATH_UPDATED=0

# Configure bash
if configure_path "$HOME/.bashrc" "bash"; then
    PATH_UPDATED=1
fi

# Configure zsh
if configure_path "$HOME/.zshrc" "zsh"; then
    PATH_UPDATED=1
fi

echo ""

# Check if draft is already in PATH
if command -v draft >/dev/null 2>&1; then
    INSTALLED_VERSION=$(draft --version 2>/dev/null || echo "unknown")
    echo "✓ Draft is ready to use!"
    echo "  Version: $INSTALLED_VERSION"
else
    echo "⚠ Draft is installed but not in your current PATH"
    echo ""
    echo "To use draft immediately, run:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
    echo ""
    if [ $PATH_UPDATED -eq 1 ]; then
        echo "Or restart your shell to load the updated configuration."
    fi
fi

echo ""
echo "Get started:"
echo "  cd your-project"
echo "  draft init"
echo ""
echo "Learn more: https://github.com/heiko-braun/draft"
