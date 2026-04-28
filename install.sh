#!/bin/bash
#
# devcfg installation script
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/I3-rett/devcfg/main/install.sh | bash
#
# This script tries to download a pre-built binary from GitHub Releases.
# If no release is available, it falls back to building from source (requires Go 1.24+).

set -e

REPO="I3-rett/devcfg"
BINARY_NAME="devcfg"

# Print a clean error box and exit
error_exit() {
    local msg="$1"
    echo ""
    echo "┌─────────────────────────────────────────────┐"
    echo "│  ✗ Installation failed                      │"
    printf "│  %-45s│\n" "$msg"
    echo "└─────────────────────────────────────────────┘"
    echo ""
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)
            OS_SUFFIX="linux"
            ;;
        Darwin*)
            OS_SUFFIX="darwin"
            ;;
        *)
            error_exit "OS '$OS' is not supported (linux/darwin only)"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH_SUFFIX="amd64"
            ;;
        arm64|aarch64)
            ARCH_SUFFIX="arm64"
            ;;
        *)
            error_exit "Architecture '$ARCH' is not supported (x86_64/arm64 only)"
            ;;
    esac

    PLATFORM="${OS_SUFFIX}-${ARCH_SUFFIX}"
    echo "Detected platform: $PLATFORM"
}

# Try to download pre-built binary
download_binary() {
    local url="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}-${PLATFORM}"
    local dest="$WORK_DIR/$BINARY_NAME"

    echo "Attempting to download pre-built binary from:"
    echo "  $url"

    # Use -f to fail on HTTP errors (like 404)
    if curl -fsSL "$url" -o "$dest"; then
        chmod +x "$dest"
        echo "✓ Successfully downloaded $BINARY_NAME"
        return 0
    else
        return 1
    fi
}

# Build from source
build_from_source() {
    echo "Building from source..."

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        error_exit "Go is not installed. Get it at https://go.dev/dl/"
    fi

    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Found Go version: $GO_VERSION"

    # Clone the repository
    TEMP_DIR=$(mktemp -d)
    echo "Cloning repository to $TEMP_DIR..."

    if ! git clone "https://github.com/${REPO}.git" "$TEMP_DIR" 2>/dev/null; then
        error_exit "Failed to clone repo. Check your internet connection."
        rm -rf "$TEMP_DIR"
    fi

    # Build the binary
    echo "Building binary..."
    cd "$TEMP_DIR"

    if go build -o "$WORK_DIR/$BINARY_NAME" .; then
        cd "$OLDPWD"
        rm -rf "$TEMP_DIR"
        chmod +x "$WORK_DIR/$BINARY_NAME"
        echo "✓ Successfully built $BINARY_NAME"
        return 0
    else
        cd "$OLDPWD"
        rm -rf "$TEMP_DIR"
        error_exit "Build failed. Check the output above for details."
    fi
}

# Install binary to ~/.local/bin and update PATH
install_globally() {
    local install_dir="$HOME/.local/bin"
    local src="$WORK_DIR/$BINARY_NAME"

    echo "Installing to $install_dir..."

    # Create directory if it doesn't exist
    mkdir -p "$install_dir"

    # Move binary to install directory
    mv "$src" "$install_dir/$BINARY_NAME"

    echo "✓ Installed $BINARY_NAME to $install_dir"

    # Check if already in PATH
    if echo "$PATH" | grep -q "$install_dir"; then
        echo ""
        echo "You can now run: $BINARY_NAME"
        return 0
    fi

    # Detect shell and update PATH
    local shell_rc=""
    local added_to_path=false

    # Try to detect shell from current environment
    if [ -n "$ZSH_VERSION" ]; then
        shell_rc="$HOME/.zshrc"
    elif [ -n "$BASH_VERSION" ]; then
        if [ -f "$HOME/.bashrc" ]; then
            shell_rc="$HOME/.bashrc"
        else
            shell_rc="$HOME/.bash_profile"
        fi
    elif [ -n "$FISH_VERSION" ]; then
        shell_rc="$HOME/.config/fish/config.fish"
    else
        # Fallback: detect from $SHELL
        case "$SHELL" in
            */zsh)
                shell_rc="$HOME/.zshrc"
                ;;
            */bash)
                if [ -f "$HOME/.bashrc" ]; then
                    shell_rc="$HOME/.bashrc"
                else
                    shell_rc="$HOME/.bash_profile"
                fi
                ;;
            */fish)
                shell_rc="$HOME/.config/fish/config.fish"
                ;;
        esac
    fi

    # Add to PATH if shell was detected
    if [ -n "$shell_rc" ]; then
        # Check if PATH export already exists in rc file
        if [ -f "$shell_rc" ] && grep -q "$install_dir" "$shell_rc" 2>/dev/null; then
            echo ""
            echo "✓ $install_dir is already in your shell configuration"
        else
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$shell_rc"
            added_to_path=true
            echo ""
            echo "✓ Added $install_dir to PATH in $shell_rc"
        fi
    fi

    # Provide clear instructions
    echo ""
    if [ "$added_to_path" = true ]; then
        echo "To use $BINARY_NAME globally, either:"
        echo "  1. Open a new terminal, or"
        echo "  2. Run: source $shell_rc"
        echo ""
        echo "Then you can run: $BINARY_NAME"
    elif [ -n "$shell_rc" ]; then
        echo "Restart your terminal or run: source $shell_rc"
        echo "Then you can run: $BINARY_NAME"
    else
        echo "Add $install_dir to your PATH, then you can run: $BINARY_NAME"
    fi
}

# Main installation flow
main() {
    echo "=== devcfg installer ==="
    echo ""

    WORK_DIR=$(mktemp -d)
    trap 'rm -rf "$WORK_DIR"' EXIT

    detect_platform
    echo ""

    if download_binary; then
        echo ""
        install_globally
    else
        echo "⚠ No pre-built binary available for this platform/version"
        echo ""
        build_from_source
        echo ""
        install_globally
    fi

    echo ""
    echo "Installation complete!"
}

main
