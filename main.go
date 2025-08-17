package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-wireguard-vpn/internal/config"
	"tui-wireguard-vpn/internal/ui"
	"tui-wireguard-vpn/internal/vpn"
)

var (
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))

	// Panel styles for 4-panel layout
	mainPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(1).
		MarginRight(1)

	inputPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(1)

	outputPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(1).
		MarginTop(1)

	statusPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(1).
		MarginBottom(1)

	controlsPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(1).
		MarginTop(1).
		MarginLeft(1)

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#007ACC"))

	// Active panel highlighting style
	activePanelBorder = lipgloss.Color("#007ACC")
	normalPanelBorder = lipgloss.Color("#FFFFFF")
	connectedStatusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#28A745")).
		Padding(1, 2)

	disconnectedStatusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#DC3545")).
		Padding(1, 2)

	disabledStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))
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
	title          string
	status         *vpn.ConnectionStatus
	choices        []string
	cursor         int
	vpnSvc         vpn.Service
	loading        bool
	message        string
	// 4-panel layout fields
	activePanel    int    // 0: main+status, 1: help/input, 2: activity log, 3: controls
	showInputPanel bool   // whether to show the input panel
	inputModel     *ui.UpdateModel // for configuration updates
	outputLog      []string // log messages for output panel
	terminalWidth  int
	terminalHeight int
	// Activity log scrolling
	logViewportStart int // First visible log entry
	logViewportSize  int // Number of log entries visible at once
}

func initialModel() model {
	return model{
		title:  "╭─────────────────────────╮\n│  WireGuard VPN Manager  │\n╰─────────────────────────╯",
		status: &vpn.ConnectionStatus{Connected: false},
		choices: []string{
			"Start Production VPN",
			"Start Non-Production VPN", 
			"Stop VPN",
			"Refresh Status",
			"Update VPN Configuration",
			"Quit",
		},
		cursor:         0,
		vpnSvc:         vpn.NewService(),
		loading:        false,
		message:        "",
		activePanel:    0,    // start with main menu active
		showInputPanel: false,
		outputLog:        []string{},
		terminalWidth:    80,  // default values
		terminalHeight:   24,
		logViewportStart: 0,
		logViewportSize:  5,   // Show 5 log entries at once
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
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		
		// Pass window size to input model if it exists
		if m.inputModel != nil {
			var cmd tea.Cmd
			inputModel, cmd := m.inputModel.Update(msg)
			if updatedModel, ok := inputModel.(*ui.UpdateModel); ok {
				m.inputModel = updatedModel
			}
			return m, cmd
		}
		return m, nil
		
	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			// Cycle through panels: 0 (main+status) -> 1 (help/input) -> 2 (activity) -> 3 (controls) -> 0
			m.activePanel = (m.activePanel + 1) % 4
			return m, nil
		case "esc":
			// Close input panel if open, otherwise quit
			if m.showInputPanel {
				m.showInputPanel = false
				m.activePanel = 0
				m.inputModel = nil
				m.addLogEntry("❌ Configuration update cancelled")
				return m, nil
			}
			return m, tea.Quit
		case "up", "k":
			if m.activePanel == 0 && m.cursor > 0 {
				// Main menu navigation
				m.cursor--
			} else if m.activePanel == 2 {
				// Activity log scrolling up
				if m.logViewportStart > 0 {
					m.logViewportStart--
				}
			}
		case "down", "j":
			if m.activePanel == 0 && m.cursor < len(m.choices)-1 {
				// Main menu navigation
				m.cursor++
			} else if m.activePanel == 2 {
				// Activity log scrolling down
				maxStart := len(m.outputLog) - 5 // Use constant viewport size for simplicity
				if maxStart < 0 {
					maxStart = 0
				}
				if m.logViewportStart < maxStart {
					m.logViewportStart++
				}
			}
		case "enter", " ":
			// Only handle menu selection when main panel is active AND no input panel is showing
			if m.activePanel != 0 || m.showInputPanel {
				break
			}
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
				// Show input panel with embedded filepicker
				m.showInputPanel = true
				m.activePanel = 1 // Switch to input panel
				m.inputModel = ui.NewUpdateModel()
				m.addLogEntry("🔧 Configuration update started...")
				
				// Initialize the input model and send it a window size message
				initCmd := m.inputModel.Init()
				sizeCmd := func() tea.Msg {
					return tea.WindowSizeMsg{Width: m.terminalWidth, Height: m.terminalHeight}
				}
				return m, tea.Batch(initCmd, sizeCmd)
			case 5: // Quit
				return m, tea.Quit
			}
		}
		
		// Delegate input to input model when input panel is active
		if m.showInputPanel && m.activePanel == 1 && m.inputModel != nil {
			var cmd tea.Cmd
			inputModel, cmd := m.inputModel.Update(msg)
			if updatedModel, ok := inputModel.(*ui.UpdateModel); ok {
				m.inputModel = updatedModel
				
				// Check if input model has a config path (user completed selection)
				if configPath := m.inputModel.GetConfigPath(); configPath != "" {
					// Start config update process
					m.showInputPanel = false
					m.activePanel = 0
					m.inputModel = nil
					m.loading = true
					m.message = "Updating configuration..."
					m.addLogEntry(fmt.Sprintf("🔧 Processing config: %s", configPath))
					return m, updateConfig(m.vpnSvc, configPath)
				}
			}
			return m, cmd
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
				m.addLogEntry("✅ Configuration updated successfully!")
			case "start_Production":
				m.message = "✅ Production VPN started successfully!"
				m.addLogEntry("✅ Production VPN started successfully!")
			case "start_NonProduction":
				m.message = "✅ Non-Production VPN started successfully!"
				m.addLogEntry("✅ Non-Production VPN started successfully!")
			case "stop":
				m.message = "✅ VPN stopped successfully!"
				m.addLogEntry("✅ VPN stopped successfully!")
			default:
				m.message = fmt.Sprintf("Operation %s completed successfully", msg.operation)
				m.addLogEntry(fmt.Sprintf("Operation %s completed successfully", msg.operation))
			}
			// Refresh status after successful operation
			return m, checkVPNStatus(m.vpnSvc)
		} else {
			switch msg.operation {
			case "update_config":
				m.message = fmt.Sprintf("❌ Configuration update failed: %v", msg.err)
				m.addLogEntry(fmt.Sprintf("❌ Configuration update failed: %v", msg.err))
			case "start_Production":
				m.message = fmt.Sprintf("❌ Failed to start Production VPN: %v", msg.err)
				m.addLogEntry(fmt.Sprintf("❌ Failed to start Production VPN: %v", msg.err))
			case "start_NonProduction":
				m.message = fmt.Sprintf("❌ Failed to start Non-Production VPN: %v", msg.err)
				m.addLogEntry(fmt.Sprintf("❌ Failed to start Non-Production VPN: %v", msg.err))
			case "stop":
				m.message = fmt.Sprintf("❌ Failed to stop VPN: %v", msg.err)
				m.addLogEntry(fmt.Sprintf("❌ Failed to stop VPN: %v", msg.err))
			default:
				m.message = fmt.Sprintf("Operation %s failed: %v", msg.operation, msg.err)
				m.addLogEntry(fmt.Sprintf("Operation %s failed: %v", msg.operation, msg.err))
			}
		}
	}
	
	return m, nil
}

// addLogEntry adds a new entry to the activity log and adjusts viewport to show latest entries
func (m *model) addLogEntry(entry string) {
	m.outputLog = append(m.outputLog, entry)
	
	// Auto-scroll to show the latest entry (keep showing the most recent)
	if len(m.outputLog) > m.logViewportSize {
		m.logViewportStart = len(m.outputLog) - m.logViewportSize
	} else {
		m.logViewportStart = 0
	}
}

func (m model) View() string {
	// Simplified 4-panel layout with better proportions
	leftWidth := m.terminalWidth / 2
	rightWidth := m.terminalWidth / 2 - 2
	bottomLeftWidth := (m.terminalWidth * 2 / 3) - 1
	bottomRightWidth := (m.terminalWidth / 3) - 1
	
	topHeight := (m.terminalHeight * 2 / 3) - 6
	bottomHeight := (m.terminalHeight / 3) - 3
	
	if m.showInputPanel && m.inputModel != nil {
		// Layout with input panel: Menu + Status | Input | Activity Log | Controls
		leftPanel := m.buildMainStatusPanel(leftWidth, topHeight)
		inputPanel := m.buildInputPanel(rightWidth, topHeight)
		activityPanel := m.buildOutputPanel(bottomLeftWidth, bottomHeight)
		controlsPanel := m.buildControlsPanel(bottomRightWidth, bottomHeight)
		
		// Top row: Combined Menu+Status | Input
		topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, inputPanel)
		
		// Bottom row: Activity Log | Controls
		bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, activityPanel, controlsPanel)
		
		layout := lipgloss.JoinVertical(lipgloss.Left, 
			titleStyle.Render(m.title),
			"",
			topRow,
			"",
			bottomRow)
		
		return layout
	} else {
		// Standard layout: Menu + Status | Help | Activity Log | Controls
		leftPanel := m.buildMainStatusPanel(leftWidth, topHeight)
		helpPanel := m.buildHelpPanel(rightWidth, topHeight)
		activityPanel := m.buildOutputPanel(bottomLeftWidth, bottomHeight)
		controlsPanel := m.buildControlsPanel(bottomRightWidth, bottomHeight)
		
		// Top row: Combined Menu+Status | Help
		topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, helpPanel)
		
		// Bottom row: Activity Log | Controls
		bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, activityPanel, controlsPanel)
		
		layout := lipgloss.JoinVertical(lipgloss.Left, 
			titleStyle.Render(m.title),
			"",
			topRow,
			"",
			bottomRow)
		
		return layout
	}
}

func (m model) buildMainStatusPanel(width, height int) string {
	var content strings.Builder
	
	// VPN Status section first
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
	
	if m.status != nil && m.status.Connected {
		content.WriteString(connectedStatusStyle.Render("Status: "+statusText) + "\n")
	} else {
		content.WriteString(disconnectedStatusStyle.Render("Status: "+statusText) + "\n")
	}
	
	// Show connection details if connected
	if m.status != nil && m.status.Connected {
		if m.status.Endpoint != "" {
			content.WriteString(fmt.Sprintf("Endpoint: %s\n", m.status.Endpoint))
		}
		if m.status.LastSeen != nil {
			content.WriteString(fmt.Sprintf("Last Handshake: %s ago\n", time.Since(*m.status.LastSeen).Truncate(time.Second)))
		}
		if m.status.BytesRx > 0 || m.status.BytesTx > 0 {
			content.WriteString(fmt.Sprintf("Data: ↓ %s  ↑ %s\n", formatBytes(m.status.BytesRx), formatBytes(m.status.BytesTx)))
		}
	}
	
	content.WriteString("\n🎛️  Main Menu\n")
	content.WriteString("─────────────────────\n")
	
	// Menu
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i && m.activePanel == 0 {
			cursor = ">"
		}
		
		// Disable certain options based on state
		disabled := false
		if m.status != nil {
			if i == 0 && m.status.Connected && m.status.Environment == vpn.Production {
				disabled = true
			}
			if i == 1 && m.status.Connected && m.status.Environment == vpn.NonProduction {
				disabled = true
			}
			if i == 2 && !m.status.Connected {
				disabled = true
			}
		} else if i == 2 {
			disabled = true
		}
		
		style := ""
		if disabled {
			style = disabledStyle.Render(fmt.Sprintf("%s %s (disabled)", cursor, choice))
		} else if m.loading && m.cursor == i {
			style = fmt.Sprintf("%s %s (loading...)", cursor, choice)
		} else if m.cursor == i && m.activePanel == 0 {
			style = selectedStyle.Render(fmt.Sprintf("%s %s", cursor, choice))
		} else {
			style = fmt.Sprintf("%s %s", cursor, choice)
		}
		
		content.WriteString(style + "\n")
	}
	
	// Message area
	if m.message != "" {
		content.WriteString("\n" + m.message + "\n")
	}
	
	panelStyle := mainPanelStyle.Width(width).Height(height)
	if m.activePanel == 0 {
		panelStyle = panelStyle.BorderForeground(activePanelBorder) // Blue for active panel
	} else {
		panelStyle = panelStyle.BorderForeground(normalPanelBorder) // White for inactive panel
	}
	
	return panelStyle.Render(content.String())
}

func (m model) buildStatusPanel(width, height int) string {
	var content strings.Builder
	
	content.WriteString("📊 VPN Status\n\n")
	
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
	
	content.WriteString(statusStyle.Render("Status: "+statusText) + "\n\n")
	
	// Show additional connection details if connected
	if m.status != nil && m.status.Connected {
		if m.status.Endpoint != "" {
			content.WriteString(fmt.Sprintf("Endpoint: %s\n", m.status.Endpoint))
		}
		if m.status.LastSeen != nil {
			content.WriteString(fmt.Sprintf("Last Handshake: %s ago\n", time.Since(*m.status.LastSeen).Truncate(time.Second)))
		}
		if m.status.BytesRx > 0 || m.status.BytesTx > 0 {
			content.WriteString(fmt.Sprintf("Data: ↓ %s  ↑ %s\n", formatBytes(m.status.BytesRx), formatBytes(m.status.BytesTx)))
		}
	} else {
		content.WriteString("No active VPN connection\n")
		content.WriteString("Select a VPN option from the menu\n")
	}
	
	return statusPanelStyle.Width(width).Height(height).Render(content.String())
}

func (m model) buildInputPanel(width, height int) string {
	if m.inputModel == nil {
		return m.buildHelpPanel(width, height)
	}
	
	// Get the input model view without panel styling first
	inputView := m.inputModel.View()
	
	// Apply minimal panel styling that doesn't constrain content
	panelStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(1)
		
	if m.activePanel == 1 {
		panelStyle = panelStyle.BorderForeground(activePanelBorder) // Blue for active panel
	} else {
		panelStyle = panelStyle.BorderForeground(normalPanelBorder) // White for inactive panel
	}
	
	return panelStyle.Render(inputView)
}

func (m model) buildHelpPanel(width, height int) string {
	helpText := `🔧 Configuration Panel

File picker for config selection:
• Use ↑/↓ to navigate files
• Enter to select/enter directories  
• h = Home directory
• Ctrl+H = Toggle hidden files
• Select .conf files to proceed

Tab to switch between panels
Esc to close panels`

	panelStyle := inputPanelStyle.Width(width).Height(height).BorderForeground(normalPanelBorder)
	return panelStyle.Render(helpText)
}

func (m model) buildOutputPanel(width, height int) string {
	var content strings.Builder
	
	// Calculate viewport size based on panel height
	viewportSize := height - 5 // Account for title, separator and borders
	if viewportSize < 1 {
		viewportSize = 1
	}
	
	// Panel title with focus indicator
	title := "📊 Activity Log"
	if m.activePanel == 2 {
		title = "📊 Activity Log (Press ↑/↓ to scroll, Tab to switch panels)"
		content.WriteString(selectedStyle.Render(title) + "\n")
	} else {
		content.WriteString(title + "\n")
	}
	content.WriteString("──────────────────────────────────────────────────────────────────────────\n")
	
	if len(m.outputLog) == 0 {
		content.WriteString("No activity yet. Start by using the VPN controls above.\n")
	} else {
		// Calculate viewport
		endIdx := m.logViewportStart + viewportSize
		if endIdx > len(m.outputLog) {
			endIdx = len(m.outputLog)
		}
		
		// Show scroll indicators
		if m.logViewportStart > 0 {
			content.WriteString("  ↑ (more entries above)\n")
		}
		
		// Show viewport entries
		for i := m.logViewportStart; i < endIdx; i++ {
			// Clean up the log entry and ensure it fits
			logEntry := strings.TrimSpace(m.outputLog[i])
			if len(logEntry) > width-6 { // Account for borders and prefix
				logEntry = logEntry[:width-9] + "..."
			}
			content.WriteString(fmt.Sprintf("• %s\n", logEntry))
		}
		
		// Show bottom scroll indicator
		if endIdx < len(m.outputLog) {
			content.WriteString("  ↓ (more entries below)\n")
		}
		
		// Show position indicator
		if len(m.outputLog) > viewportSize {
			content.WriteString(fmt.Sprintf("Showing %d-%d of %d entries", 
				m.logViewportStart+1, endIdx, len(m.outputLog)))
		}
	}
	
	// Apply focus styling to panel border
	panelStyle := outputPanelStyle.Width(width).Height(height)
	if m.activePanel == 2 {
		panelStyle = panelStyle.BorderForeground(activePanelBorder) // Blue when focused
	} else {
		panelStyle = panelStyle.BorderForeground(normalPanelBorder) // White when not focused
	}
	
	return panelStyle.Render(content.String())
}

func (m model) buildControlsPanel(width, height int) string {
	var content strings.Builder
	
	content.WriteString("🎮 Controls\n")
	content.WriteString("──────────────────────\n")
	
	// Show controls based on active panel
	switch m.activePanel {
	case 0: // Main+Status panel
		content.WriteString("Menu + Status:\n")
		content.WriteString("• ↑/↓ - Navigate menu\n")
		content.WriteString("• Enter - Select option\n")
		content.WriteString("• Tab - Switch panels\n")
		content.WriteString("• View VPN status\n")
	case 1: // Help/Input panel
		if m.showInputPanel {
			content.WriteString("File Browser:\n")
			content.WriteString("• ↑/↓ - Navigate files\n")
			content.WriteString("• Enter - Select/Enter dir\n")
			content.WriteString("• h - Home directory\n")
			content.WriteString("• Ctrl+H - Toggle hidden\n")
			content.WriteString("• Esc - Cancel\n")
		} else {
			content.WriteString("Help Panel:\n")
			content.WriteString("• Tab - Switch panels\n")
			content.WriteString("• Information only\n")
		}
	case 2: // Activity log
		content.WriteString("Activity Log:\n")
		content.WriteString("• ↑/↓ - Scroll log\n")
		content.WriteString("• Tab - Switch panels\n")
		content.WriteString("• View operation history\n")
	case 3: // Controls panel
		content.WriteString("Controls Panel:\n")
		content.WriteString("• View only\n")
		content.WriteString("• Tab - Switch panels\n")
		content.WriteString("• Context help\n")
	}
	
	content.WriteString("\nGlobal:\n")
	content.WriteString("• q/Ctrl+C - Quit\n")
	content.WriteString("• Tab - Cycle panels\n")
	
	panelStyle := controlsPanelStyle.Width(width).Height(height)
	if m.activePanel == 3 {
		panelStyle = panelStyle.BorderForeground(activePanelBorder) // Blue when focused
	} else {
		panelStyle = panelStyle.BorderForeground(normalPanelBorder) // White when not focused
	}
	return panelStyle.Render(content.String())
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