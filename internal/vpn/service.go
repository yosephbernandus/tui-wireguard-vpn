package vpn

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"tui-wireguard-vpn/internal/config"
)

type WireGuardService struct{}

func NewService() *WireGuardService {
	return &WireGuardService{}
}

func (w *WireGuardService) GetStatus() (*ConnectionStatus, error) {
	cmd := exec.Command("wg", "show")
	output, err := cmd.Output()
	if err != nil {
		return &ConnectionStatus{Connected: false}, nil
	}

	// Look for JULO VPN interfaces specifically, prioritize active ones
	var juloInterfaces []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "interface:") {
			interfaceName := strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			// Only consider JULO interfaces
			if strings.HasPrefix(interfaceName, "julo-") {
				juloInterfaces = append(juloInterfaces, interfaceName)
			}
		}
	}
	
	// If no JULO interfaces found, return disconnected
	if len(juloInterfaces) == 0 {
		return &ConnectionStatus{Connected: false}, nil
	}
	
	// If multiple interfaces, we have a problem - stop the extras and use the first
	if len(juloInterfaces) > 1 {
		// Stop all but the first interface silently
		for i := 1; i < len(juloInterfaces); i++ {
			cmd := exec.Command("wg-quick", "down", juloInterfaces[i])
			cmd.Run() // Ignore errors, just try to clean up
		}
		// Use the first interface after cleanup (don't recurse)
	}
	
	// Get detailed status for the first (and should be only) interface
	activeInterface := juloInterfaces[0]
	return w.getInterfaceStatus(activeInterface)
}

func (w *WireGuardService) getInterfaceStatus(interfaceName string) (*ConnectionStatus, error) {
	cmd := exec.Command("wg", "show", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return &ConnectionStatus{Connected: false}, nil
	}

	status := &ConnectionStatus{
		Connected: true,
		Interface: interfaceName,
	}
	
	// Determine environment from interface name
	if strings.Contains(interfaceName, "nonprod") {
		status.Environment = NonProduction
	} else if strings.Contains(interfaceName, "prod") {
		status.Environment = Production
	}
	
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "endpoint:") {
			status.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
		}
		
		if strings.HasPrefix(line, "latest handshake:") {
			handshakeStr := strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			if handshakeStr != "" && handshakeStr != "0" {
				if t, err := parseHandshakeTime(handshakeStr); err == nil {
					status.LastSeen = &t
				}
			}
		}
		
		if strings.HasPrefix(line, "transfer:") {
			transferStr := strings.TrimSpace(strings.TrimPrefix(line, "transfer:"))
			parts := strings.Split(transferStr, ",")
			if len(parts) >= 2 {
				if rx, err := parseBytes(strings.TrimSpace(parts[0])); err == nil {
					status.BytesRx = rx
				}
				if tx, err := parseBytes(strings.TrimSpace(parts[1])); err == nil {
					status.BytesTx = tx
				}
			}
		}
	}
	
	return status, nil
}

func (w *WireGuardService) Start(env Environment) error {
	// First, check if any VPN is currently running and stop it
	status, err := w.GetStatus()
	if err == nil && status.Connected {
		// Stop current VPN silently - the TUI will handle the messaging
		if stopErr := w.Stop(); stopErr != nil {
			return fmt.Errorf("failed to stop current VPN (%s): %v", status.Interface, stopErr)
		}
	}
	
	configName := fmt.Sprintf("julo-%s", string(env))
	cmd := exec.Command("wg-quick", "up", configName)
	
	// Capture both stdout and stderr to see what failed
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick up %s failed: %v\nOutput: %s", configName, err, string(output))
	}
	return nil
}

func (w *WireGuardService) Stop() error {
	status, err := w.GetStatus()
	if err != nil {
		return err
	}
	
	if !status.Connected {
		return nil
	}
	
	// Try to stop the detected interface
	interfaceName := status.Interface
	if interfaceName == "" {
		// Fallback: try both possible interfaces
		for _, iface := range []string{"julo-prod", "julo-nonprod"} {
			cmd := exec.Command("wg-quick", "down", iface)
			_, err := cmd.CombinedOutput()
			if err == nil {
				return nil // Successfully stopped
			}
			// Continue trying other interfaces silently
		}
		return fmt.Errorf("no active VPN interfaces found to stop")
	}
	
	cmd := exec.Command("wg-quick", "down", interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick down %s failed: %v\nOutput: %s", interfaceName, err, string(output))
	}
	return nil
}

func (w *WireGuardService) UpdateConfig(userConfigPath string) error {
	if userConfigPath == "" {
		return fmt.Errorf("user config file path is required")
	}
	
	// Use the same logic as the original j1-vpn-update-config script
	processor := config.NewConfigProcessor()
	return processor.ProcessUserConfigDirectly(userConfigPath)
}

func (w *WireGuardService) GetConfig(env Environment) (string, error) {
	configName := fmt.Sprintf("julo-%s.conf", string(env))
	configPath := fmt.Sprintf("/etc/wireguard/%s", configName)
	
	// Read the config file
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file %s: %v", configPath, err)
	}
	
	// Filter out sensitive information
	lines := strings.Split(string(content), "\n")
	var filteredLines []string
	
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		
		// Filter out sensitive keys but keep other config
		if strings.HasPrefix(trimmedLine, "PrivateKey") ||
		   strings.HasPrefix(trimmedLine, "PresharedKey") ||
		   strings.HasPrefix(trimmedLine, "PublicKey") {
			// Show field name but hide the actual key
			parts := strings.SplitN(trimmedLine, "=", 2)
			if len(parts) == 2 {
				filteredLines = append(filteredLines, fmt.Sprintf("%s = [HIDDEN]", strings.TrimSpace(parts[0])))
			}
		} else if strings.HasPrefix(trimmedLine, "AllowedIPs") {
			// Format AllowedIPs with proper line breaks for better readability
			parts := strings.SplitN(trimmedLine, "=", 2)
			if len(parts) == 2 {
				filteredLines = append(filteredLines, strings.TrimSpace(parts[0])+" =")
				// Split IPs by comma and show each on a new line with indentation
				ips := strings.Split(strings.TrimSpace(parts[1]), ",")
				for i, ip := range ips {
					cleanIP := strings.TrimSpace(ip)
					if i == 0 {
						filteredLines = append(filteredLines, fmt.Sprintf("  %s", cleanIP))
					} else {
						filteredLines = append(filteredLines, fmt.Sprintf("  %s", cleanIP))
					}
				}
			}
		} else {
			// Show all other configuration lines
			filteredLines = append(filteredLines, trimmedLine)
		}
	}
	
	return strings.Join(filteredLines, "\n"), nil
}

func parseHandshakeTime(handshakeStr string) (time.Time, error) {
	if strings.Contains(handshakeStr, "second") {
		parts := strings.Fields(handshakeStr)
		if len(parts) >= 1 {
			if seconds, err := strconv.Atoi(parts[0]); err == nil {
				return time.Now().Add(-time.Duration(seconds) * time.Second), nil
			}
		}
	}
	if strings.Contains(handshakeStr, "minute") {
		parts := strings.Fields(handshakeStr)
		if len(parts) >= 1 {
			if minutes, err := strconv.Atoi(parts[0]); err == nil {
				return time.Now().Add(-time.Duration(minutes) * time.Minute), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse handshake time: %s", handshakeStr)
}

func parseBytes(bytesStr string) (uint64, error) {
	bytesStr = strings.TrimSpace(bytesStr)
	
	multiplier := uint64(1)
	if strings.HasSuffix(bytesStr, "KiB") {
		multiplier = 1024
		bytesStr = strings.TrimSuffix(bytesStr, "KiB")
	} else if strings.HasSuffix(bytesStr, "MiB") {
		multiplier = 1024 * 1024
		bytesStr = strings.TrimSuffix(bytesStr, "MiB")
	} else if strings.HasSuffix(bytesStr, "GiB") {
		multiplier = 1024 * 1024 * 1024
		bytesStr = strings.TrimSuffix(bytesStr, "GiB")
	} else if strings.HasSuffix(bytesStr, "B") {
		bytesStr = strings.TrimSuffix(bytesStr, "B")
	}
	
	value, err := strconv.ParseFloat(bytesStr, 64)
	if err != nil {
		return 0, err
	}
	
	return uint64(value * float64(multiplier)), nil
}