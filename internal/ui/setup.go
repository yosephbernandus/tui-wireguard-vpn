package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-wireguard-vpn/internal/config"
)

var (
	setupTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#F25D94")).
		Padding(0, 1)

	setupInfoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1).
		MarginBottom(1)

	setupErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5F87"))

	setupSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B"))
)

type SetupModel struct {
	inputs        []textinput.Model
	focused       int
	setupStatus   *config.SetupStatus
	stage         int // 0: info, 1: prod config, 2: nonprod config, 3: processing, 4: complete
	message       string
	err           error
	prodPath      string
	nonprodPath   string
}

func NewSetupModel(status *config.SetupStatus) *SetupModel {
	inputs := make([]textinput.Model, 2)
	
	// Production config file input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "/path/to/your-prod-config.conf"
	inputs[0].Focus()
	inputs[0].CharLimit = 256
	inputs[0].Width = 50
	
	// Non-production config file input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "/path/to/your-nonprod-config.conf"
	inputs[1].CharLimit = 256
	inputs[1].Width = 50
	
	return &SetupModel{
		inputs:      inputs,
		focused:     0,
		setupStatus: status,
		stage:       0,
	}
}

func (m *SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ExitAndSetupMsg:
		// Store the paths and quit TUI to run setup in terminal
		m.prodPath = msg.prodPath
		m.nonprodPath = msg.nonprodPath
		return m, tea.Quit
	case SetupCompleteMsg:
		if msg.success {
			m.stage = 4 // Complete
			m.message = ""
			m.err = nil
		} else {
			m.stage = 3 // Stay in processing but show error
			m.message = fmt.Sprintf("Setup failed: %v", msg.err)
			m.err = msg.err
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.stage == 4 { // Complete
				return m, tea.Quit
			}
		case "enter":
			switch m.stage {
			case 0: // Info screen
				m.stage = 1
				return m, nil
			case 1: // Prod config input
				if strings.TrimSpace(m.inputs[0].Value()) == "" {
					m.message = "Please enter the production config file path"
					return m, nil
				}
				m.stage = 2
				m.focused = 1
				m.inputs[1].Focus()
				m.inputs[0].Blur()
				return m, nil
			case 2: // Nonprod config input  
				if strings.TrimSpace(m.inputs[1].Value()) == "" {
					m.message = "Please enter the non-production config file path"
					return m, nil
				}
				// Exit TUI and run setup in terminal
				return m, m.exitAndRunSetup()
			}
		case "tab", "shift+tab", "up", "down":
			if m.stage == 2 {
				if m.focused == 0 {
					m.focused = 1
					m.inputs[0].Blur()
					m.inputs[1].Focus()
				} else {
					m.focused = 0
					m.inputs[1].Blur()
					m.inputs[0].Focus()
				}
			}
		case "esc":
			if m.stage > 0 && m.stage < 3 {
				m.stage = 0
				m.focused = 0
				m.inputs[0].Focus()
				m.inputs[1].Blur()
				m.message = ""
			}
		}
	}

	// Handle textinput updates
	if m.stage == 1 || m.stage == 2 {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *SetupModel) View() string {
	var s strings.Builder
	
	s.WriteString(setupTitleStyle.Render("WireGuard VPN Setup Required"))
	s.WriteString("\n\n")

	switch m.stage {
	case 0: // Info screen
		s.WriteString(setupInfoStyle.Render("Initial setup is required. This will:"))
		s.WriteString("\n")
		s.WriteString("• Install WireGuard configuration templates\n")
		s.WriteString("• Process your config files\n")
		s.WriteString("• Generate production and non-production configurations\n")
		s.WriteString("\n")
		
		if len(m.setupStatus.MissingFiles) > 0 {
			s.WriteString("Missing files:\n")
			for _, file := range m.setupStatus.MissingFiles {
				s.WriteString(fmt.Sprintf("  - %s\n", file))
			}
			s.WriteString("\n")
		}
		
		s.WriteString("Press Enter to continue, Ctrl+C to quit")

	case 1: // Production config input
		s.WriteString("Step 1: Production Configuration\n\n")
		s.WriteString("Please enter the path to your production WireGuard config file:\n")
		s.WriteString("(This should contain your production private key and settings)\n\n")
		s.WriteString(m.inputs[0].View())
		s.WriteString("\n\n")
		s.WriteString("Press Enter to continue, Esc to go back")

	case 2: // Non-production config input
		s.WriteString("Step 2: Non-Production Configuration\n\n")
		s.WriteString(fmt.Sprintf("Production config: %s\n", m.inputs[0].Value()))
		s.WriteString("\nPlease enter the path to your non-production WireGuard config file:\n")
		s.WriteString("(This should contain your non-production private key and settings)\n\n")
		s.WriteString(m.inputs[1].View())
		s.WriteString("\n\n")
		s.WriteString("Use Tab to switch fields, Enter to start setup, Esc to go back")

	case 3: // Processing
		s.WriteString("Processing configuration files...\n\n")
		s.WriteString("This requires sudo privileges to write to /etc/wireguard/\n")

	case 4: // Complete
		s.WriteString(setupSuccessStyle.Render("Setup Complete!"))
		s.WriteString("\n\n")
		s.WriteString("Configuration files have been installed successfully.\n")
		s.WriteString("You can now use the VPN management features.\n\n")
		s.WriteString("Press q to continue to main application")
	}

	if m.message != "" {
		s.WriteString("\n")
		if m.err != nil {
			s.WriteString(setupErrorStyle.Render(m.message))
		} else {
			s.WriteString(m.message)
		}
	}

	return s.String()
}

func (m *SetupModel) exitAndRunSetup() tea.Cmd {
	return func() tea.Msg {
		return ExitAndSetupMsg{
			prodPath:    strings.TrimSpace(m.inputs[0].Value()),
			nonprodPath: strings.TrimSpace(m.inputs[1].Value()),
		}
	}
}

type SetupCompleteMsg struct {
	success bool
	err     error
}

type ExitAndSetupMsg struct {
	prodPath    string
	nonprodPath string
}

func (m *SetupModel) GetConfigPaths() (string, string) {
	return m.prodPath, m.nonprodPath
}