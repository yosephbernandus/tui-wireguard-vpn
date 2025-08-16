package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-wireguard-vpn/internal/config"
	"tui-wireguard-vpn/internal/ui"
	"tui-wireguard-vpn/internal/vpn"
)

var (
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))
)

type vpnStatusMsg struct {
	status *vpn.ConnectionStatus
	err    error
}

type vpnOperationMsg struct {
	operation string
	success   bool
	err       error
}

type model struct {
	title     string
	status    *vpn.ConnectionStatus
	choices   []string
	cursor    int
	vpnSvc    vpn.Service
	loading   bool
	message   string
}

func initialModel() model {
	return model{
		title:  "WireGuard VPN Manager",
		status: &vpn.ConnectionStatus{Connected: false},
		choices: []string{
			"Start Production VPN",
			"Start Non-Production VPN", 
			"Stop VPN",
			"Refresh Status",
			"Update Configuration",
			"Quit",
		},
		cursor:  0,
		vpnSvc:  vpn.NewService(),
		loading: false,
		message: "",
	}
}

func checkVPNStatus(svc vpn.Service) tea.Cmd {
	return func() tea.Msg {
		status, err := svc.GetStatus()
		return vpnStatusMsg{status: status, err: err}
	}
}

func startVPN(svc vpn.Service, env vpn.Environment) tea.Cmd {
	return func() tea.Msg {
		err := svc.Start(env)
		return vpnOperationMsg{
			operation: fmt.Sprintf("start_%s", string(env)),
			success:   err == nil,
			err:       err,
		}
	}
}

func stopVPN(svc vpn.Service) tea.Cmd {
	return func() tea.Msg {
		err := svc.Stop()
		return vpnOperationMsg{
			operation: "stop",
			success:   err == nil,
			err:       err,
		}
	}
}

func updateConfig(svc vpn.Service, configPath string) tea.Cmd {
	return func() tea.Msg {
		err := svc.UpdateConfig(configPath)
		return vpnOperationMsg{
			operation: "update_config",
			success:   err == nil,
			err:       err,
		}
	}
}

func (m model) Init() tea.Cmd {
	return checkVPNStatus(m.vpnSvc)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", " ":
			switch m.cursor {
			case 0: // Start Production VPN
				m.loading = true
				if m.status != nil && m.status.Connected {
					m.message = "Switching to Production VPN..."
				} else {
					m.message = "Starting Production VPN..."
				}
				return m, startVPN(m.vpnSvc, vpn.Production)
			case 1: // Start Non-Production VPN
				m.loading = true
				if m.status != nil && m.status.Connected {
					m.message = "Switching to Non-Production VPN..."
				} else {
					m.message = "Starting Non-Production VPN..."
				}
				return m, startVPN(m.vpnSvc, vpn.NonProduction)
			case 2: // Stop VPN
				m.loading = true
				m.message = "Stopping VPN..."
				return m, stopVPN(m.vpnSvc)
			case 3: // Refresh Status
				m.loading = true
				m.message = "Checking VPN status..."
				return m, checkVPNStatus(m.vpnSvc)
			case 4: // Update Configuration
				// Switch to update input mode
				updateModel := ui.NewUpdateModel()
				p := tea.NewProgram(updateModel)
				finalModel, err := p.Run()
				if err != nil {
					m.message = fmt.Sprintf("Error running update: %v", err)
					return m, nil
				}
				
				// Check if user provided config path
				if updateModelFinal, ok := finalModel.(*ui.UpdateModel); ok {
					configPath := updateModelFinal.GetConfigPath()
					if configPath != "" {
						// Start update process with loading
						m.loading = true
						m.message = "Updating configuration..."
						return m, updateConfig(m.vpnSvc, configPath)
					}
				}
				return m, nil
			case 5: // Quit
				return m, tea.Quit
			}
		}
		
	case vpnStatusMsg:
		m.loading = false
		if msg.err != nil {
			m.message = fmt.Sprintf("Error checking status: %v", msg.err)
		} else {
			m.status = msg.status
			m.message = "Status updated"
		}
		
	case vpnOperationMsg:
		m.loading = false
		if msg.success {
			switch msg.operation {
			case "update_config":
				m.message = "✅ Configuration updated successfully!"
			case "start_Production":
				m.message = "✅ Production VPN started successfully!"
			case "start_NonProduction":
				m.message = "✅ Non-Production VPN started successfully!"
			case "stop":
				m.message = "✅ VPN stopped successfully!"
			default:
				m.message = fmt.Sprintf("Operation %s completed successfully", msg.operation)
			}
			// Refresh status after successful operation
			return m, checkVPNStatus(m.vpnSvc)
		} else {
			switch msg.operation {
			case "update_config":
				m.message = fmt.Sprintf("❌ Configuration update failed: %v", msg.err)
			case "start_Production":
				m.message = fmt.Sprintf("❌ Failed to start Production VPN: %v", msg.err)
			case "start_NonProduction":
				m.message = fmt.Sprintf("❌ Failed to start Non-Production VPN: %v", msg.err)
			case "stop":
				m.message = fmt.Sprintf("❌ Failed to stop VPN: %v", msg.err)
			default:
				m.message = fmt.Sprintf("Operation %s failed: %v", msg.operation, msg.err)
			}
		}
	}
	
	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render(m.title) + "\n\n"
	
	// VPN Status section
	statusText := "Disconnected"
	if m.status != nil && m.status.Connected {
		env := "Unknown"
		if m.status.Environment == vpn.Production {
			env = "Production"
		} else if m.status.Environment == vpn.NonProduction {
			env = "Non-Production"
		}
		statusText = fmt.Sprintf("Connected to %s", env)
		if m.status.Interface != "" {
			statusText += fmt.Sprintf(" (%s)", m.status.Interface)
		}
	}
	
	s += statusStyle.Render("Status: "+statusText) + "\n\n"
	
	// Show additional connection details if connected
	if m.status != nil && m.status.Connected {
		if m.status.Endpoint != "" {
			s += fmt.Sprintf("Endpoint: %s\n", m.status.Endpoint)
		}
		if m.status.LastSeen != nil {
			s += fmt.Sprintf("Last Handshake: %s ago\n", time.Since(*m.status.LastSeen).Truncate(time.Second))
		}
		if m.status.BytesRx > 0 || m.status.BytesTx > 0 {
			s += fmt.Sprintf("Data: ↓ %s  ↑ %s\n", formatBytes(m.status.BytesRx), formatBytes(m.status.BytesTx))
		}
		s += "\n"
	}
	
	// Menu
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		
		// Disable certain options based on state
		disabled := false
		if m.status != nil {
			// Only disable the VPN type that's currently running
			if i == 0 && m.status.Connected && m.status.Environment == vpn.Production {
				disabled = true // Disable "Start Production VPN" if Production is already running
			}
			if i == 1 && m.status.Connected && m.status.Environment == vpn.NonProduction {
				disabled = true // Disable "Start Non-Production VPN" if Non-Production is already running
			}
			if i == 2 && !m.status.Connected {
				disabled = true // Disable "Stop VPN" if no VPN is running
			}
		} else {
			// If status is nil, disable stop option
			if i == 2 {
				disabled = true
			}
		}
		
		style := ""
		if disabled {
			style = helpStyle.Render(fmt.Sprintf("%s %s (disabled)", cursor, choice))
		} else if m.loading && m.cursor == i {
			style = fmt.Sprintf("%s %s (loading...)", cursor, choice)
		} else {
			style = fmt.Sprintf("%s %s", cursor, choice)
		}
		
		s += style + "\n"
	}
	
	// Message area
	if m.message != "" {
		s += "\n" + m.message + "\n"
	}
	
	s += "\n" + helpStyle.Render("Use ↑/↓ or j/k to navigate, Enter to select, q to quit")
	return s
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
	// Handle command-line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := installToSystem(); err != nil {
				fmt.Printf("Installation failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Installation completed successfully!")
			fmt.Println("You can now run 'tui-wireguard-vpn' from anywhere.")
			return
		case "setup":
			// Handle setup mode for processing configs with sudo
			if err := handleSetupMode(); err != nil {
				fmt.Printf("Setup failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "update-config":
			// Handle single config update mode
			if len(os.Args) < 3 {
				fmt.Printf("Usage: %s update-config <config-file>\n", os.Args[0])
				os.Exit(1)
			}
			if err := handleUpdateConfigMode(os.Args[2]); err != nil {
				fmt.Printf("Config update failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Check if we need initial setup
	setupStatus, err := config.CheckSetupStatus()
	if err != nil {
		fmt.Printf("Error checking setup status: %v\n", err)
		os.Exit(1)
	}

	// If setup is needed, start with setup screen
	if setupStatus.NeedsSetup {
		setupModel := ui.NewSetupModel(setupStatus)
		p := tea.NewProgram(setupModel)
		finalModel, err := p.Run()
		if err != nil {
			fmt.Printf("Error running setup: %v", err)
			os.Exit(1)
		}
		
		// Check if user completed config input and we need to run setup
		if setupModelFinal, ok := finalModel.(*ui.SetupModel); ok {
			prodPath, nonprodPath := setupModelFinal.GetConfigPaths()
			if prodPath != "" || nonprodPath != "" {
				// Exit TUI and run setup in terminal
				fmt.Println("\nStarting VPN configuration setup...")
				fmt.Println("This process requires sudo privileges to write to /etc/wireguard/")
				fmt.Println("")
				
				if err := config.RunSetupDirectly(prodPath, nonprodPath); err != nil {
					fmt.Printf("Setup failed: %v\n", err)
					os.Exit(1)
				}
				
				fmt.Println("\n✅ Setup completed successfully!")
				fmt.Println("You can now run 'tui-wireguard-vpn' to manage your VPN connections.")
				return
			}
		}
		return
	}

	// Normal operation - start main VPN management UI
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

func installToSystem() error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	targetPath := "/usr/local/bin/tui-wireguard-vpn"

	// Copy executable to /usr/local/bin
	sourceFile, err := os.Open(execPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file (try running with sudo): %v", err)
	}
	defer targetFile.Close()

	if _, err := targetFile.ReadFrom(sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	// Set executable permissions
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %v", err)
	}

	return nil
}

func handleSetupMode() error {
	// This handles the sudo setup process when called with "setup" argument
	// Parse additional arguments for config file paths
	var prodConfigPath, nonprodConfigPath string
	
	fmt.Printf("Setup mode: Processing arguments: %v\n", os.Args)
	
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--prod":
			if i+1 < len(os.Args) {
				prodConfigPath = os.Args[i+1]
				fmt.Printf("Production config: %s\n", prodConfigPath)
				i++
			}
		case "--nonprod":
			if i+1 < len(os.Args) {
				nonprodConfigPath = os.Args[i+1]
				fmt.Printf("Non-production config: %s\n", nonprodConfigPath)
				i++
			}
		}
	}

	// Validate config files exist
	if prodConfigPath != "" {
		if _, err := os.Stat(prodConfigPath); os.IsNotExist(err) {
			return fmt.Errorf("production config file not found: %s", prodConfigPath)
		}
	}
	
	if nonprodConfigPath != "" {
		if _, err := os.Stat(nonprodConfigPath); os.IsNotExist(err) {
			return fmt.Errorf("non-production config file not found: %s", nonprodConfigPath)
		}
	}

	// Run the setup process
	processor := config.NewConfigProcessor()
	return processor.RunSetup(prodConfigPath, nonprodConfigPath)
}

func handleUpdateConfigMode(userConfigPath string) error {
	// This handles the sudo config update process when called with "update-config" argument
	fmt.Printf("Update config mode: Processing config file: %s\n", userConfigPath)
	
	// Validate config file exists
	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", userConfigPath)
	}

	// Run the config update process (same as original j1-vpn-update-config)
	processor := config.NewConfigProcessor()
	return processor.ProcessUserConfig(userConfigPath)
}