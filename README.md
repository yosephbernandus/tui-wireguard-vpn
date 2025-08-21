# WireGuard VPN TUI

Terminal User Interface (TUI) for managing WireGuard VPN connections. Replace bash scripts with an intuitive panel interface.

## Features

- **Panel Interface** - Menu, Configuration, Activity Log, and Controls
- **VPN Switching** - Toggle between Production and Non-Production environments
- **Integrated File Browser** - Navigate and select config files
- **Config Viewing** - View VPN configurations
- **Quick Setup** - Guided initial configuration process
- **Cross-Platform** - Works on Linux and macOS
- **Passwordless Operation** - Optional sudoers configuration for seamless usage
- **Shell Integration** - Customizable aliases for quick access

## Prerequisites

**Before installing this application, you must have WireGuard installed on your system.**

### Install WireGuard First

**Linux:**
```bash
# Ubuntu/Debian
sudo apt update && sudo apt install wireguard

# CentOS/RHEL
sudo yum install wireguard-tools

# Fedora
sudo dnf install wireguard-tools

# Arch Linux
sudo pacman -S wireguard-tools
```

**macOS:**
```bash
brew install wireguard-tools
```

### Verify Installation
```bash
# Check if WireGuard is installed
wg version
wg-quick --help
```

**Note:** This TUI application is a frontend for WireGuard. It requires the `wg` and `wg-quick` commands to manage VPN connections.

## Screenshots

```
╭─────────────────╮
│ WireGuard VPN   │
╰─────────────────╯

┌──────────────────────────┬─────────────────────────┐
│ Status: Connected (prod) │ Configuration Panel     │
│                          │                         │
│ Main Menu                │ File picker for config  │
│ > Start Production VPN   │ • Use ↑/↓ to navigate   │
│   Start Non-Prod VPN     │ • Enter to select       │
│   Stop VPN               │ • h = Home directory    │
│   Refresh Status         │                         │
│   Update Configuration   │                         │
│   View Production Config │                         │
│   Quit                   │                         │
└──────────────────────────┴─────────────────────────┘
┌──────────────────────────┬─────────────────────────┐
│ Activity Log             │ Controls                │
│ • VPN connected to prod  │ • ↑/↓ - Navigate        │
│ • Config updated         │ • Enter - Select        │
│ • Status refreshed       │ • Tab - Switch panels   │
└──────────────────────────┴─────────────────────────┘
```

## Installation

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/yosephbernandus/tui-wireguard-vpn/master/scripts/install.sh | sudo bash
```

This will:
- Download the correct binary for your system
- Install to `/usr/local/bin/`
- Set up passwordless sudo (optional)
- Configure shell alias (optional)

### Manual Installation

1. **Download the binary** for your platform from [releases](https://github.com/yosephbernandus/tui-wireguard-vpn/releases)
2. **Make it executable:**
   ```bash
   chmod +x tui-wireguard-vpn-*
   ```
3. **Move to system path:**
   ```bash
   sudo mv tui-wireguard-vpn-* /usr/local/bin/tui-wireguard-vpn
   ```

### Development Installation

```bash
# Clone the repository
git clone https://github.com/yosephbernandus/tui-wireguard-vpn.git
cd tui-wireguard-vpn

# Build from source
go build -o tui-wireguard-vpn

# Install to system
sudo ./tui-wireguard-vpn install
```

## System Requirements

- **Operating System:** Linux or macOS
- **WireGuard:** Must be installed (`wg` and `wg-quick` commands available)
- **Privileges:** sudo/admin access for VPN operations
- **Config Files:** Your own WireGuard configuration files (.conf format)

### Installing WireGuard

**Linux:**
```bash
# Ubuntu/Debian
sudo apt update && sudo apt install wireguard

# CentOS/RHEL
sudo yum install wireguard-tools

# Fedora
sudo dnf install wireguard-tools

# Arch Linux
sudo pacman -S wireguard-tools
```

**macOS:**
```bash
brew install wireguard-tools
```

## Usage

### First Time Setup

1. **Run the application:**
   ```bash
   sudo tui-wireguard-vpn
   # or if you set up an alias:
   vpn
   ```

2. **Initial configuration** (guided process):
   - Choose config selection method (text input or file browser)
   - Select your **production** WireGuard config file
   - Select your **non-production** WireGuard config file
   - Setup completes automatically

3. **Start managing VPN connections** using the intuitive interface

### Daily Usage

```bash
# If you set up the alias during installation
vpn

# Or use the full command
sudo tui-wireguard-vpn

# If you configured passwordless sudo, no password needed!
tui-wireguard-vpn
```

### Controls

- **↑/↓** - Navigate menus and lists
- **Enter** - Select option or confirm
- **Tab** - Switch between panels
- **h** - Go to home directory (in file browser)
- **Ctrl+H** - Toggle hidden files (in file browser)
- **Esc** - Go back or close panels
- **q/Ctrl+C** - Quit application

## Configuration

The application manages WireGuard configurations by:

1. **Processing your config files** and installing them to `/etc/wireguard/`
2. **Preserving your private settings** while updating network configurations
3. **Enabling easy switching** between environments

### Config File Structure

Your WireGuard config should follow standard format:
```ini
[Interface]
PrivateKey = your-private-key
Address = 10.0.0.2/24
DNS = 1.1.1.1

[Peer]
PublicKey = peer-public-key
Endpoint = vpn.example.com:51820
AllowedIPs = 0.0.0.0/0
```

## Features in Detail

### 4-Panel Layout
- **Top Left:** VPN status and main menu
- **Top Right:** Configuration panel (help or file browser)
- **Bottom Left:** Activity log with scrolling
- **Bottom Right:** Context-sensitive controls

### VPN Operations
- **Start Production VPN** - Connect to production environment
- **Start Non-Production VPN** - Connect to staging/dev environment
- **Stop VPN** - Disconnect from any active VPN
- **Refresh Status** - Update connection status
- **Update Configuration** - Modify VPN settings
- **View Configurations** - Display config details (keys hidden)

### Security Features
- **Private key protection** - Never displays sensitive keys
- **Sudo integration** - Secure privilege escalation
- **Config validation** - Ensures proper WireGuard format
- **Safe file handling** - Prevents accidental overwrites

## Supported Platforms

| Platform | Architectures | Status |
|----------|---------------|--------|
| Linux    | x64, ARM64, x86 | ✅ Supported |
| macOS    | Intel, Apple Silicon | ✅ Supported |
| Windows  | - | ❌ Not supported |

## Troubleshooting

### Common Issues

**"wg command not found"**
```bash
# Install WireGuard tools (see System Requirements)
```

**"Permission denied"**
```bash
# Make sure to run with sudo or set up passwordless sudo
sudo tui-wireguard-vpn
```

**"Config file not found"**
- Ensure your `.conf` files are accessible
- Use the file browser to navigate to correct location
- Check file permissions

**"Interface already exists"**
```bash
# Stop any existing VPN connections
sudo wg-quick down julo-prod
sudo wg-quick down julo-nonprod
```

### Debug Mode

Set environment variable for verbose output:
```bash
DEBUG=1 sudo tui-wireguard-vpn
```

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/yosephbernandus/tui-wireguard-vpn.git
cd tui-wireguard-vpn

# Install dependencies
go mod tidy

# Build
go build -o tui-wireguard-vpn

# Run tests
go test ./...

# Build for all platforms
./scripts/build-release.sh v1.0.0
```

### Project Structure

```
tui-wireguard-vpn/
├── main.go                 # Main application entry point
├── internal/
│   ├── vpn/               # VPN service and operations
│   ├── ui/                # UI components and models
│   └── config/            # Configuration management
├── scripts/
│   ├── build-release.sh   # Cross-platform build script
│   ├── install.sh         # Installation script
│   └── release.sh         # Release automation
└── README.md
```

