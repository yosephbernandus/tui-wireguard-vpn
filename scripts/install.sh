#!/bin/bash
# install.sh - WireGuard VPN TUI Installer

set -e

# Configuration
REPO="yosephbernandus/tui-wireguard-vpn"  # Repo
VERSION="latest"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="tui-wireguard-vpn"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
note() { echo -e "${BLUE}[NOTE]${NC} $1"; }

# Banner
show_banner() {
    echo ""
    echo "╭─────────────────────────────────────╮"
    echo "│      WireGuard VPN TUI Installer    │"
    echo "╰─────────────────────────────────────╯"
    echo ""
}

# Detect system architecture and OS
detect_system() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|cygwin*|msys*) OS="windows" ;;
        *) error "Unsupported OS: $os"; exit 1 ;;
    esac
    
    case $arch in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        i386|i686) ARCH="386" ;;
        *) error "Unsupported architecture: $arch"; exit 1 ;;
    esac
    
    BINARY_FILE="${BINARY_NAME}-${OS}-${ARCH}"
    if [[ "$OS" == "windows" ]]; then
        BINARY_FILE="${BINARY_FILE}.exe"
    fi
    
    info "Detected system: ${OS} ${ARCH}"
}

# Check if running with proper privileges
check_privileges() {
    info "Checking installation privileges..."
    
    if [[ $EUID -ne 0 ]]; then
        error "This installer requires sudo privileges"
        echo ""
        echo "Please run with sudo:"
        echo "  curl -sSL https://raw.githubusercontent.com/${REPO}/master/scripts/install.sh | sudo bash"
        echo ""
        echo "Or download and run manually:"
        echo "  wget https://raw.githubusercontent.com/${REPO}/master/scripts/install.sh"
        echo "  chmod +x install.sh"
        echo "  sudo ./install.sh"
        exit 1
    fi
    
    info "✓ Running with sudo privileges"
}

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."
    
    # Check WireGuard
    if ! command -v wg &> /dev/null; then
        warn "WireGuard not found!"
        echo ""
        echo "Please install WireGuard first:"
        case $OS in
            linux)
                echo "  Ubuntu/Debian: apt update && apt install wireguard"
                echo "  CentOS/RHEL:   yum install wireguard-tools"
                echo "  Fedora:        dnf install wireguard-tools"
                echo "  Arch:          pacman -S wireguard-tools"
                ;;
            darwin)
                echo "  macOS:         brew install wireguard-tools"
                ;;
        esac
        echo ""
        error "Install WireGuard and run this installer again"
        exit 1
    fi
    
    info "✓ WireGuard found"
    
    # Check required tools
    if ! command -v curl &> /dev/null && ! command -v wget &> /dev/null; then
        error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    info "✓ Download tools available"
}

# Download binary
download_binary() {
    info "Downloading ${BINARY_FILE}..."
    
    local temp_dir=$(mktemp -d)
    local download_url="https://github.com/${REPO}/releases/latest/download/${BINARY_FILE}"
    
    # Try download
    if command -v curl &> /dev/null; then
        if ! curl -sSL --fail -o "${temp_dir}/${BINARY_NAME}" "$download_url"; then
            error "Failed to download binary from $download_url"
            error "Please check if the release exists and the binary name is correct"
            exit 1
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q -O "${temp_dir}/${BINARY_NAME}" "$download_url"; then
            error "Failed to download binary from $download_url"
            error "Please check if the release exists and the binary name is correct"
            exit 1
        fi
    fi
    
    # Verify download
    if [[ ! -f "${temp_dir}/${BINARY_NAME}" ]]; then
        error "Binary file not found after download"
        exit 1
    fi
    
    # Check if it's an actual binary (not HTML error page)
    if file "${temp_dir}/${BINARY_NAME}" | grep -q "HTML"; then
        error "Downloaded file appears to be HTML (probably 404 error)"
        error "Please check if the release exists: https://github.com/${REPO}/releases"
        exit 1
    fi
    
    # Install binary
    chmod +x "${temp_dir}/${BINARY_NAME}"
    mv "${temp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    
    info "✓ Binary installed to ${INSTALL_DIR}/${BINARY_NAME}"
    
    # Cleanup
    rm -rf "$temp_dir"
}

# Setup sudoers for passwordless execution
setup_sudoers() {
    info "Setting up passwordless sudo..."
    
    echo -n "Setup passwordless sudo to run VPN without password? (Y/n): "
    read -r setup_sudo < /dev/tty
    
    # Default to yes if empty
    setup_sudo=${setup_sudo:-Y}
    
    if [[ $setup_sudo =~ ^[Yy]$ ]]; then
        local sudoers_file="/etc/sudoers.d/wireguard-vpn"
        local actual_user="${SUDO_USER:-$USER}"
        
        info "Detected user: $actual_user"
        
        # Remove old sudoers file if exists
        [[ -f "$sudoers_file" ]] && rm -f "$sudoers_file"
        
        # Always create user-specific sudoers entry (like your preference)
        # This allows running both wg-quick and the TUI app without password
        # SETENV allows preserving environment variables needed for TUI display
        cat > "$sudoers_file" << EOF
# Allow $actual_user to run WireGuard VPN TUI without password
$actual_user ALL=(ALL) NOPASSWD:SETENV: ${INSTALL_DIR}/${BINARY_NAME}
$actual_user ALL=(ALL) NOPASSWD: /usr/bin/wg-quick
$actual_user ALL=(ALL) NOPASSWD: /usr/bin/wg
EOF
        
        # Set proper permissions
        chmod 440 "$sudoers_file"
        
        # Validate sudoers syntax
        if ! visudo -c -f "$sudoers_file" >/dev/null 2>&1; then
            error "Sudoers configuration syntax error"
            rm -f "$sudoers_file"
            return 1
        fi
        
        info "✓ Passwordless sudo configured for user: $actual_user"
        info "✓ Added permissions for: ${INSTALL_DIR}/${BINARY_NAME}, wg-quick, wg"
        PASSWORDLESS_SUDO=true
    else
        info "Skipping sudoers setup - you'll need to use 'sudo' when running VPN commands"
        PASSWORDLESS_SUDO=false
    fi
}

# Setup shell alias
setup_alias() {
    info "Setting up shell alias..."
    
    local actual_user="${SUDO_USER:-$USER}"
    local user_home=$(eval echo "~$actual_user")
    
    # Ask for alias name
    echo -n "Enter alias name for VPN manager (default: vpn): "
    read -r alias_name < /dev/tty
    alias_name=${alias_name:-vpn}
    
    # Find shell config files
    local shell_configs=()
    [[ -f "$user_home/.zshrc" ]] && shell_configs+=("$user_home/.zshrc")
    [[ -f "$user_home/.bashrc" ]] && shell_configs+=("$user_home/.bashrc")
    [[ -f "$user_home/.bash_profile" ]] && shell_configs+=("$user_home/.bash_profile")
    
    # macOS specific
    if [[ "$OS" == "darwin" && -f "$user_home/.zprofile" ]]; then
        shell_configs+=("$user_home/.zprofile")
    fi
    
    if [[ ${#shell_configs[@]} -eq 0 ]]; then
        warn "No shell config found."
        echo "Add this alias manually to your shell config:"
        if [[ "$PASSWORDLESS_SUDO" == "true" ]]; then
            echo "  alias ${alias_name}='${INSTALL_DIR}/${BINARY_NAME}'"
        else
            echo "  alias ${alias_name}='sudo ${INSTALL_DIR}/${BINARY_NAME}'"
        fi
        return
    fi
    
    # Choose config file
    local shell_config
    if [[ ${#shell_configs[@]} -gt 1 ]]; then
        echo "Multiple shell configs found:"
        for i in "${!shell_configs[@]}"; do
            echo "  $((i+1)). ${shell_configs[i]}"
        done
        echo -n "Choose config file (1-${#shell_configs[@]}, default: 1): "
        read -r choice < /dev/tty
        choice=${choice:-1}
        shell_config="${shell_configs[$((choice-1))]}"
    else
        shell_config="${shell_configs[0]}"
    fi
    
    # Create alias (use sudo since setup operations require root privileges)
    local alias_cmd="sudo ${INSTALL_DIR}/${BINARY_NAME}"
    
    local alias_line="alias ${alias_name}='${alias_cmd}'"
    
    # Check if alias already exists
    if grep -q "alias ${alias_name}=" "$shell_config" 2>/dev/null; then
        warn "Alias '${alias_name}' already exists in ${shell_config}"
        echo -n "Overwrite? (y/N): "
        read -r overwrite < /dev/tty
        if [[ $overwrite =~ ^[Yy]$ ]]; then
            sed -i.bak "/alias ${alias_name}=/d" "$shell_config"
            echo "$alias_line" >> "$shell_config"
            info "✓ Alias updated in ${shell_config}"
        else
            info "Keeping existing alias"
        fi
    else
        echo "$alias_line" >> "$shell_config"
        info "✓ Alias '${alias_name}' added to ${shell_config}"
    fi
    
    # Fix ownership
    chown "$actual_user:$(id -gn "$actual_user")" "$shell_config"
    
    ALIAS_NAME="$alias_name"
}

# Show completion message
show_completion() {
    echo ""
    echo "Installation successfully!"
    echo ""
    echo "╭─────────────────────────────────────╮"
    echo "│            Usage Options            │"
    echo "╰─────────────────────────────────────╯"
    echo ""
    
    if [[ -n "$ALIAS_NAME" ]]; then
        echo "Quick start:"
        if [[ "$PASSWORDLESS_SUDO" == "true" ]]; then
            echo "  ${ALIAS_NAME}                    # Start VPN manager"
        else
            echo "  ${ALIAS_NAME}                    # Start VPN manager (will ask for password)"
        fi
        echo ""
        echo "After restarting your terminal or run:"
        echo "  source $(eval echo "~${SUDO_USER:-$USER}")/.zshrc   # if using zsh"
        echo "  source $(eval echo "~${SUDO_USER:-$USER}")/.bashrc  # if using bash"
        echo ""
    fi
    
    echo "Direct command:"
    if [[ "$PASSWORDLESS_SUDO" == "true" ]]; then
        echo "  ${INSTALL_DIR}/${BINARY_NAME}       # No password required"
    else
        echo "  sudo ${INSTALL_DIR}/${BINARY_NAME}  # Password required"
    fi
    echo ""
    
    echo "First run will guide you through VPN setup:"
    echo "  • Select your production config file"
    echo "  • Select your non-production config file"
    echo "  • Automatic installation and configuration"
    echo "  • Please restart your terminal or open a new terminal"
    echo ""
    
    echo "Documentation:"
    echo "  https://github.com/${REPO}#readme"
    echo ""
}

# Main installation function
main() {
    show_banner
    detect_system
    check_privileges
    check_prerequisites
    download_binary
    setup_sudoers
    setup_alias
    show_completion
}

# Run main installation
main "$@"
