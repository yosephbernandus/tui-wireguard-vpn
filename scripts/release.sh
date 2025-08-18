#!/bin/bash
# scripts/release.sh - Create git release without GitHub CLI

set -e

VERSION=${1}
if [[ -z "$VERSION" ]]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

REPO="yosephbernandus/tui-wireguard-vpn"  # Change this
BUILD_DIR="release"

echo "Creating git release ${VERSION}..."

# Build binaries first
echo "Building binaries..."
./scripts/build-release.sh "$VERSION"

# Check if tag exists
if git tag -l | grep -q "^${VERSION}$"; then
    echo "WARNING: Tag ${VERSION} already exists"
    echo -n "Delete and recreate? (y/N): "
    read -r recreate
    if [[ $recreate =~ ^[Yy]$ ]]; then
        git tag -d "$VERSION"
        git push origin --delete "$VERSION" 2>/dev/null || true
    else
        echo "ERROR: Aborting release"
        exit 1
    fi
fi

# Create and push tag
echo "Creating git tag..."
git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

echo ""
echo "SUCCESS: Git tag $VERSION created and pushed successfully!"
echo ""
echo "Next steps (manual):"
echo "1. Go to: https://github.com/${REPO}/releases/new"
echo "2. Select tag: $VERSION"
echo "3. Set release title: 'WireGuard VPN TUI $VERSION'"
echo "4. Upload these binaries from ./$BUILD_DIR/:"
ls -la "$BUILD_DIR"/ | grep -v "^total" | grep -v "^d"
echo ""
echo "5. Use this release description:"
echo ""

# Generate release notes for copy-paste
cat << EOF
## WireGuard VPN TUI ${VERSION}

Cross-platform Terminal User Interface for WireGuard VPN management.

### Features
- Beautiful 4-panel TUI interface
- Easy VPN switching (Production <-> Non-Production)
- Integrated file browser for config selection
- Secure config viewing (private keys hidden)
- Quick setup and configuration management

### Quick Install

**One-liner install (recommended):**
\`\`\`bash
curl -sSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sudo bash
\`\`\`

**Manual install:**
1. Download the appropriate binary for your system
2. Place it in \`/usr/local/bin/\`
3. Make it executable: \`chmod +x /usr/local/bin/tui-wireguard-vpn\`

### System Requirements
- Linux or macOS
- WireGuard installed (\`wg\` and \`wg-quick\` commands)
- sudo/admin privileges for VPN operations
- Your own WireGuard config files (.conf format)

### Usage
\`\`\`bash
# After installation with alias setup
vpn

# Or direct command
sudo tui-wireguard-vpn
\`\`\`

### Supported Platforms
- **Linux:** x64, ARM64, x86
- **macOS:** Intel (x64), Apple Silicon (ARM64)

### First Time Setup
1. Run the application
2. Choose config file selection method (text input or file browser)
3. Select your production WireGuard config
4. Select your non-production WireGuard config
5. Setup completes automatically
6. Start managing your VPN connections!

### What's New in ${VERSION}
- Initial release with full VPN management capabilities
- Cross-platform binary support
- Automated installation script
- Passwordless sudo configuration
- Shell alias setup

---
**Full documentation:** https://github.com/${REPO}#readme
EOF

echo ""
echo "6. Publish the release"
echo ""
echo "Install command for users (after release is published):"
echo "curl -sSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sudo bash"
