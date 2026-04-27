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
            echo "Error: Unsupported operating system: $OS"
            exit 1
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
            echo "Error: Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    PLATFORM="${OS_SUFFIX}-${ARCH_SUFFIX}"
    echo "Detected platform: $PLATFORM"
}

# Try to download pre-built binary
download_binary() {
    local url="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}-${PLATFORM}"

    echo "Attempting to download pre-built binary from:"
    echo "  $url"

    # Use -f to fail on HTTP errors (like 404)
    if curl -fsSL "$url" -o "$BINARY_NAME" 2>/dev/null; then
        chmod +x "$BINARY_NAME"
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
        echo "Error: Go is not installed. Please install Go 1.24+ from https://go.dev/dl/"
        exit 1
    fi

    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Found Go version: $GO_VERSION"

    # Clone the repository
    TEMP_DIR=$(mktemp -d)
    echo "Cloning repository to $TEMP_DIR..."

    if ! git clone "https://github.com/${REPO}.git" "$TEMP_DIR" 2>/dev/null; then
        echo "Error: Failed to clone repository. Please check your internet connection."
        rm -rf "$TEMP_DIR"
        exit 1
    fi

    # Build the binary
    echo "Building binary..."
    cd "$TEMP_DIR"

    if go build -o "$BINARY_NAME" .; then
        # Move binary to original directory
        mv "$BINARY_NAME" "$OLDPWD/$BINARY_NAME"
        cd "$OLDPWD"
        rm -rf "$TEMP_DIR"
        chmod +x "$BINARY_NAME"
        echo "✓ Successfully built $BINARY_NAME"
        return 0
    else
        echo "Error: Failed to build from source"
        cd "$OLDPWD"
        rm -rf "$TEMP_DIR"
        exit 1
    fi
}

# Install binary to ~/.local/bin and update PATH
install_globally() {
    local install_dir="$HOME/.local/bin"

    echo "Installing to $install_dir..."

    # Create directory if it doesn't exist
    mkdir -p "$install_dir"

    # Move binary to install directory
    mv "$BINARY_NAME" "$install_dir/$BINARY_NAME"

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
