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

# Main installation flow
main() {
    echo "=== devcfg installer ==="
    echo ""

    detect_platform
    echo ""

    if download_binary; then
        echo ""
        echo "Installation complete!"
        echo "Run with: ./$BINARY_NAME"
    else
        echo "⚠ No pre-built binary available for this platform/version"
        echo ""
        build_from_source
        echo ""
        echo "Installation complete!"
        echo "Run with: ./$BINARY_NAME"
    fi
}

main
