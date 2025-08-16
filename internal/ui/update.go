package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	updateTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#BD93F9")).
		Padding(0, 1)

	updateInfoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1).
		MarginBottom(1)

	updateErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5F87"))

	updateSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B"))
)

type UpdateModel struct {
	filepicker  filepicker.Model
	textinput   textinput.Model
	stage       int // 0: info, 1: choose mode, 2: text input, 3: file picker, 4: processing, 5: complete
	inputMode   int // 0: text input, 1: file browser
	message     string
	err         error
	configPath  string
}

func NewUpdateModel() *UpdateModel {
	// Setup filepicker with better visibility
	fp := filepicker.New()
	// Don't filter by file type initially - we'll validate on selection
	fp.AllowedTypes = []string{}
	fp.ShowHidden = true  // Show hidden directories and files
	fp.DirAllowed = true
	fp.FileAllowed = true
	fp.AutoHeight = false
	fp.Height = 15 // Show more entries
	fp.ShowPermissions = true
	fp.ShowSize = true
	
	// Get current working directory
	currentDir, _ := os.Getwd()
	fp.CurrentDirectory = currentDir
	
	// Try to start in common config locations with .conf files
	commonDirs := []string{
		currentDir, // Current directory first
		filepath.Join(os.Getenv("HOME"), "Downloads"),
		filepath.Join(os.Getenv("HOME"), ".ssh"),
		"/etc/wireguard",
		os.Getenv("HOME"), // Home directory
	}
	
	for _, dir := range commonDirs {
		if _, err := os.Stat(dir); err == nil {
			// Check if this directory has .conf files
			files, err := filepath.Glob(filepath.Join(dir, "*.conf"))
			if err == nil && len(files) > 0 {
				fp.CurrentDirectory = dir
				break
			}
			// If no .conf files, at least use the first accessible directory
			if fp.CurrentDirectory == "" {
				fp.CurrentDirectory = dir
			}
		}
	}

	// Setup text input
	ti := textinput.New()
	ti.Placeholder = "/path/to/config.conf"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return &UpdateModel{
		filepicker: fp,
		textinput:  ti,
		stage:      0,
		inputMode:  0,
	}
}

func (m *UpdateModel) Init() tea.Cmd {
	return tea.Batch(m.filepicker.Init(), textinput.Blink)
}

func (m *UpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.stage == 0 {
				return m, tea.Quit
			}
			// Go back to previous stage
			if m.stage > 0 {
				m.stage--
			}
			return m, nil
		case "enter":
			switch m.stage {
			case 0: // Info screen
				m.stage = 1
				return m, nil
			case 1: // Choose mode screen
				if m.inputMode == 0 {
					m.stage = 2 // Text input
					m.textinput.Focus()
				} else {
					m.stage = 3 // File picker
				}
				return m, nil
			case 2: // Text input mode
				path := strings.TrimSpace(m.textinput.Value())
				if path == "" {
					m.message = "Please enter a file path"
					return m, nil
				}
				// Validate file exists and has .conf extension
				if !strings.HasSuffix(path, ".conf") {
					m.message = "Please select a .conf file"
					return m, nil
				}
				if _, err := os.Stat(path); os.IsNotExist(err) {
					m.message = "File does not exist"
					return m, nil
				}
				m.configPath = path
				return m, tea.Quit
			}
		case "esc":
			if m.stage > 0 {
				m.stage--
				m.message = ""
				return m, nil
			} else {
				return m, tea.Quit
			}
		case "1":
			if m.stage == 1 { // Choose mode screen
				m.inputMode = 0 // Text input
				return m, nil
			}
		case "2":
			if m.stage == 1 { // Choose mode screen
				m.inputMode = 1 // File browser
				return m, nil
			}
		case "tab":
			if m.stage == 1 { // Choose mode screen
				m.inputMode = 1 - m.inputMode // Toggle between 0 and 1
				return m, nil
			}
		case "h":
			// Go to home directory in file picker
			if m.stage == 3 {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					m.filepicker.CurrentDirectory = homeDir
					return m, m.filepicker.Init()
				}
			}
		case "~":
			// Go to home directory in file picker
			if m.stage == 3 {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					m.filepicker.CurrentDirectory = homeDir
					return m, m.filepicker.Init()
				}
			}
		case ".":
			// Go to current working directory in file picker
			if m.stage == 3 {
				currentDir, _ := os.Getwd()
				m.filepicker.CurrentDirectory = currentDir
				return m, m.filepicker.Init()
			}
		case "ctrl+h":
			// Toggle hidden files visibility
			if m.stage == 3 {
				m.filepicker.ShowHidden = !m.filepicker.ShowHidden
				return m, m.filepicker.Init()
			}
		}
	}

	// Handle text input updates when in text input mode
	if m.stage == 2 {
		var cmd tea.Cmd
		m.textinput, cmd = m.textinput.Update(msg)
		return m, cmd
	}

	// Handle filepicker updates when in file picker mode
	if m.stage == 3 {
		var cmd tea.Cmd
		m.filepicker, cmd = m.filepicker.Update(msg)

		// Check if file was selected after updating
		if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
			// Validate that it's a .conf file
			if !strings.HasSuffix(strings.ToLower(path), ".conf") {
				m.message = "Please select a .conf file"
				return m, cmd
			}
			// File selected successfully - store path and quit
			m.configPath = path
			return m, tea.Quit
		}

		return m, cmd
	}

	return m, nil
}

func (m *UpdateModel) View() string {
	var s strings.Builder

	s.WriteString(updateTitleStyle.Render("Update VPN Configuration"))
	s.WriteString("\n\n")

	switch m.stage {
	case 0: // Info screen
		s.WriteString(updateInfoStyle.Render("This will update your VPN configuration with new settings."))
		s.WriteString("\n")
		s.WriteString("The process will:\n")
		s.WriteString("• Select your updated WireGuard config file\n")
		s.WriteString("• Detect if it's for Production or Non-Production\n")
		s.WriteString("• Merge it with the current template\n")
		s.WriteString("• Update the active configuration\n")
		s.WriteString("\n")
		s.WriteString("Press Enter to continue, Esc to cancel")

	case 1: // Choose input mode
		s.WriteString("Choose how to select your config file:\n\n")
		
		if m.inputMode == 0 {
			s.WriteString("▶ 1. Type file path manually\n")
			s.WriteString("  2. Browse files\n")
		} else {
			s.WriteString("  1. Type file path manually\n")
			s.WriteString("▶ 2. Browse files\n")
		}
		
		s.WriteString("\nUse Tab to switch, Enter to select, Esc to go back")

	case 2: // Text input mode
		s.WriteString("Enter the path to your WireGuard config file:\n\n")
		s.WriteString(m.textinput.View())
		s.WriteString("\n\nPress Enter to confirm, Esc to go back")

	case 3: // File picker
		s.WriteString("Browse for your WireGuard config file:\n")
		hiddenStatus := "Hidden files: OFF"
		if m.filepicker.ShowHidden {
			hiddenStatus = "Hidden files: ON"
		}
		s.WriteString(fmt.Sprintf("📁 Current directory: %s | %s\n", m.filepicker.CurrentDirectory, hiddenStatus))
		s.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		s.WriteString("📂 = Directory | 📄 = File | ⬆️ ⬇️ Navigate | ➡️ Enter directory | Enter = Select .conf file\n")
		s.WriteString("Shortcuts: h/~ = Home | . = Current dir | Ctrl+H = Toggle hidden files | Esc = Go back\n")
		s.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
		s.WriteString(m.filepicker.View())
		s.WriteString("\n💡 Tip: Navigate to directories with Enter, select .conf files to proceed")
	}

	if m.message != "" {
		s.WriteString("\n")
		if m.err != nil {
			s.WriteString(updateErrorStyle.Render(m.message))
		} else {
			s.WriteString(updateInfoStyle.Render(m.message))
		}
	}

	return s.String()
}

func (m *UpdateModel) GetConfigPath() string {
	return m.configPath
}