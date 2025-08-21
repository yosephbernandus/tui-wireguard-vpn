package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	// Template file contents (embedded in the application)
	prodTemplateContent = `[Interface]
 PrivateKey = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Address = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
DNS = 169.254.169.254
MTU = 1200

[Peer]
Endpoint =  34.101.166.184:51820
PresharedKey = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
PublicKey = Do4l8x0uasEPcwCPa+KdzLsgYhQtPWqifmj+2xlhxzU=
AllowedIPs = 169.254.169.254/32, 172.31.0.0/32, 10.80.0.0/16, 10.88.0.0/16, 192.168.1.95/32, 192.168.10.245/32, 192.168.11.242/32, 104.18.3.47/32, 104.18.2.47/32, 75.2.99.223/32, 99.83.238.127/32, 44.193.116.48/32, 54.157.159.41/32, 51.250.21.168/32, 89.248.204.154/32, 149.129.215.16/32, 8.215.83.84/32, 147.139.130.231/32, 8.215.78.31/32, 52.95.178.0/23, 3.5.36.0/22, 52.95.177.0/24, 108.136.154.16/28, 108.136.154.32/28, 108.136.154.48/28, 43.218.193.112/28, 43.218.193.96/28, 43.218.222.160/28, 43.218.222.176/28, 172.16.160.28/32, 172.16.160.186/32, 34.117.236.210/32
PersistentKeepAlive = 10
`

	nonprodTemplateContent = `[Interface]
 PrivateKey = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
DNS = 169.254.169.254
Address = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
MTU = 1200

[Peer]
Endpoint =  34.128.85.147:51820
PresharedKey = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
PublicKey = 1KEK7tM3wzoK6Et+xRZpNJJN33lrTvzTasTMjXx0sGk=
AllowedIPs = 172.30.0.0/16, 169.254.169.254/32, 10.88.0.0/16, 10.128.0.0/16, 192.168.1.95/32, 192.168.10.245/32, 192.168.11.242/32, 104.18.3.47/32, 104.18.2.47/32, 75.2.99.223/32, 99.83.238.127/32, 44.193.116.48/32, 54.157.159.41/32, 51.250.21.168/32, 89.248.204.154/32, 149.129.215.16/32, 8.215.83.84/32, 147.139.130.231/32, 8.215.78.31/32, 52.95.178.0/23, 3.5.36.0/22, 52.95.177.0/24, 108.136.154.16/28, 108.136.154.32/28, 108.136.154.48/28, 43.218.193.112/28, 43.218.193.96/28, 43.218.222.160/28, 43.218.222.176/28, 34.54.194.205/32, 35.241.15.137/32, 10.129.0.0/16
PersistentKeepAlive = 10
`
)

type ConfigProcessor struct{}

func NewConfigProcessor() *ConfigProcessor {
	return &ConfigProcessor{}
}

// InstallTemplates replicates "make install" - installs template files to /etc/wireguard/
func (cp *ConfigProcessor) InstallTemplates() error {
	// Create /etc/wireguard directory if it doesn't exist
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Install production template
	prodTemplatePath := filepath.Join(ConfigDir, ProdTemplate)
	if err := cp.writeFileWithContent(prodTemplatePath, prodTemplateContent); err != nil {
		return fmt.Errorf("failed to install production template: %v", err)
	}

	// Install non-production template
	nonprodTemplatePath := filepath.Join(ConfigDir, NonProdTemplate)
	if err := cp.writeFileWithContent(nonprodTemplatePath, nonprodTemplateContent); err != nil {
		return fmt.Errorf("failed to install non-production template: %v", err)
	}

	// Don't print directly - let the TUI handle the output
	// fmt.Printf("Installed templates to %s\n", ConfigDir)
	return nil
}

// ProcessUserConfig replicates "j1-vpn-update-config" behavior
func (cp *ConfigProcessor) ProcessUserConfig(userConfigPath string) error {
	// Validate user config file exists
	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("user config file not found: %s", userConfigPath)
	}

	// Read user config to detect environment by endpoint
	endpoint, err := cp.extractEndpoint(userConfigPath)
	if err != nil {
		return fmt.Errorf("failed to extract endpoint from config: %v", err)
	}

	// Determine environment based on endpoint (exactly like bash script)
	var templatePath, outputPath string

	switch endpoint {
	case ProdEndpoint:
		templatePath = filepath.Join(ConfigDir, ProdTemplate)
		outputPath = filepath.Join(ConfigDir, ProdConfig)
	case NonProdEndpoint:
		templatePath = filepath.Join(ConfigDir, NonProdTemplate)
		outputPath = filepath.Join(ConfigDir, NonProdConfig)
	default:
		return fmt.Errorf("the config you specify (%s) is not JULO's VPN config.\nPlease check with Infra Team", userConfigPath)
	}

	// Check if template exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("template file not found: %s", templatePath)
	}

	// Merge user config with template (replicating the awk script logic)
	if err := cp.updateConfig(userConfigPath, templatePath, outputPath); err != nil {
		return fmt.Errorf("failed to update config: %v", err)
	}

	// Don't print directly - let the TUI handle the output
	// fmt.Printf("Generated new config file %s\n", outputPath)
	return nil
}

// updateConfig replicates the awk script in j1-vpn-update-config
func (cp *ConfigProcessor) updateConfig(userConfigPath, templatePath, outputPath string) error {
	// Extract DNS and AllowedIPs from template (like the bash script)
	templateDNS, err := cp.extractConfigLine(templatePath, "DNS")
	if err != nil {
		return fmt.Errorf("failed to extract DNS from template: %v", err)
	}

	templateAllowedIPs, err := cp.extractConfigLine(templatePath, "AllowedIPs")
	if err != nil {
		return fmt.Errorf("failed to extract AllowedIPs from template: %v", err)
	}

	// Read user config
	userFile, err := os.Open(userConfigPath)
	if err != nil {
		return err
	}
	defer userFile.Close()

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file (try running with sudo): %v", err)
	}
	defer outputFile.Close()

	// Process user config line by line, replicating the awk script:
	// /^AllowedIPs/ { print newroute; }
	// /^DNS/ { print dns; }
	// !/^AllowedIPs/ && !/^DNS/ {print $0;}
	scanner := bufio.NewScanner(userFile)
	allowedIPsRegex := regexp.MustCompile(`^AllowedIPs`)
	dnsRegex := regexp.MustCompile(`^DNS`)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case allowedIPsRegex.MatchString(line):
			// Replace with template AllowedIPs
			fmt.Fprintln(outputFile, templateAllowedIPs)
		case dnsRegex.MatchString(line):
			// Replace with template DNS
			fmt.Fprintln(outputFile, templateDNS)
		default:
			// Keep original line
			fmt.Fprintln(outputFile, line)
		}
	}

	return scanner.Err()
}

func (cp *ConfigProcessor) extractEndpoint(configPath string) (string, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Replicate the bash awk pattern: awk '/Endpoint/ { print $3;}'
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Endpoint") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[2], nil
			}
		}
	}

	return "", fmt.Errorf("no Endpoint found in config file")
}

func (cp *ConfigProcessor) extractConfigLine(configPath, key string) (string, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Look for lines starting with the key (like grep DNS ${NEWCFG})
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key) {
			return line, nil // Return the full line
		}
	}

	return "", fmt.Errorf("key %s not found in config file", key)
}

func (cp *ConfigProcessor) writeFileWithContent(path, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

// RunSetup performs the complete setup process (like make install + j1-vpn-update-config)
func (cp *ConfigProcessor) RunSetup(prodConfigPath, nonprodConfigPath string) error {
	// Step 1: Install templates (like "make install")
	// Don't print directly - let the TUI handle the output
	// fmt.Println("Installing WireGuard configuration templates...")
	if err := cp.InstallTemplates(); err != nil {
		return fmt.Errorf("failed to install templates: %v", err)
	}

	// Step 2: Process user configs (like "j1-vpn-update-config")
	if prodConfigPath != "" {
		// Don't print directly - let the TUI handle the output
		// fmt.Println("\nProcessing production configuration...")
		if err := cp.ProcessUserConfig(prodConfigPath); err != nil {
			return fmt.Errorf("failed to process production config: %v", err)
		}
	}

	if nonprodConfigPath != "" {
		// Don't print directly - let the TUI handle the output
		// fmt.Println("\nProcessing non-production configuration...")
		if err := cp.ProcessUserConfig(nonprodConfigPath); err != nil {
			return fmt.Errorf("failed to process non-production config: %v", err)
		}
	}

	return nil
}

func RunSetupDirectly(prodConfigPath, nonprodConfigPath string) error {
	// Try to run the setup process directly, like the original bash scripts
	processor := NewConfigProcessor()
	err := processor.RunSetup(prodConfigPath, nonprodConfigPath)

	if err != nil {
		// Check if it's a permission error and provide platform-specific guidance
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "operation not permitted") ||
			strings.Contains(err.Error(), "access is denied") {
			return getSetupPermissionErrorMessage()
		}
		return err
	}
	return nil
}

func getSetupPermissionErrorMessage() error {
	var instructions string

	switch runtime.GOOS {
	case "windows":
		instructions = "Please run as Administrator:\n" +
			"Right-click Command Prompt → 'Run as administrator'\n" +
			"Then run: tui-wireguard-vpn"
	case "darwin":
		instructions = "Please run with administrator privileges:\n" +
			"sudo tui-wireguard-vpn"
	default: // linux and other unix-like systems
		instructions = "Please run with administrator privileges:\n" +
			"sudo tui-wireguard-vpn"
	}

	return fmt.Errorf("insufficient permissions to install templates and config files.\n\n%s\n\nThen run the initial setup again.", instructions)
}

func (cp *ConfigProcessor) ProcessUserConfigDirectly(userConfigPath string) error {
	// Try to run the update process directly, like the original bash scripts
	err := cp.ProcessUserConfig(userConfigPath)
	if err != nil {
		// Check if it's a permission error and provide platform-specific guidance
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "operation not permitted") ||
			strings.Contains(err.Error(), "access is denied") {
			return cp.getPermissionErrorMessage(userConfigPath)
		}
		return err
	}
	return nil
}

func (cp *ConfigProcessor) getPermissionErrorMessage(userConfigPath string) error {
	var instructions string

	switch runtime.GOOS {
	case "windows":
		instructions = "Please run as Administrator:\n" +
			"Right-click Command Prompt → 'Run as administrator'\n" +
			"Then run: tui-wireguard-vpn"
	case "darwin":
		instructions = "Please run with administrator privileges:\n" +
			"sudo tui-wireguard-vpn"
	default: // linux and other unix-like systems
		instructions = "Please run with administrator privileges:\n" +
			"sudo tui-wireguard-vpn"
	}

	return fmt.Errorf("insufficient permissions to write config files.\n\n%s\n\nThen select 'Update Configuration' again.", instructions)
}

