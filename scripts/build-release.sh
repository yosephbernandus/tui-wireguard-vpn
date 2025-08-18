#!/bin/bash
# scripts/build-release.sh - Build all platform binaries

set -e

VERSION=${1:-"v1.0.0"}
BINARY_NAME="tui-wireguard-vpn"
BUILD_DIR="release"

echo "Building ${BINARY_NAME} ${VERSION} for all platforms..."

# Clean and create release directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

echo "Building binaries..."

# Linux builds
echo "  → Linux amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "${BUILD_DIR}/${BINARY_NAME}-linux-amd64" .

echo "  → Linux arm64"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o "${BUILD_DIR}/${BINARY_NAME}-linux-arm64" .

echo "  → Linux 386"
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o "${BUILD_DIR}/${BINARY_NAME}-linux-386" .

# macOS builds
echo "  → macOS amd64 (Intel)"
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o "${BUILD_DIR}/${BINARY_NAME}-darwin-amd64" .

echo "  → macOS arm64 (Apple Silicon)"
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o "${BUILD_DIR}/${BINARY_NAME}-darwin-arm64" .

# Note: Windows support removed - focusing on Linux and macOS only

# Copy install script
echo "📋 Copying install script..."
cp scripts/install.sh "$BUILD_DIR/"

# Generate checksums
echo "Generating checksums..."
cd "$BUILD_DIR"
sha256sum * > checksums.txt
cd ..

echo ""
echo "Build completed successfully!"
echo "Files in ./${BUILD_DIR}:"
ls -la "$BUILD_DIR"
echo ""
echo "Ready for release!"
