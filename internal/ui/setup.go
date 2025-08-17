package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	stage         int // 0: info, 1: choice mode, 2: prod input method, 3: prod config, 4: nonprod input method, 5: nonprod config, 6: processing, 7: complete
	inputMode     int // 0: text input, 1: file browser
	message       string
	err           error
	prodPath      string
	nonprodPath   string
	configStep    int // 0: prod config, 1: nonprod config
	// File browser fields
	currentDir    string
	files         []os.FileInfo
	selectedIndex int
	showHidden    bool
	viewportStart int
	viewportSize  int
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
	
	// Get current working directory for file browser
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = os.Getenv("HOME")
		if currentDir == "" {
			currentDir = "."
		}
	}
	
	model := &SetupModel{
		inputs:        inputs,
		focused:       0,
		setupStatus:   status,
		stage:         0,
		inputMode:     0,
		configStep:    0,
		currentDir:    currentDir,
		selectedIndex: 0,
		showHidden:    true,
		viewportStart: 0,
		viewportSize:  10,
	}
	
	return model
}

// Add file browser functions from update.go
func (m *SetupModel) loadDirectory() error {
	file, err := os.Open(m.currentDir)
	if err != nil {
		return err
	}
	defer file.Close()

	files, err := file.Readdir(-1)
	if err != nil {
		return err
	}

	// Filter and sort files
	var filteredFiles []os.FileInfo
	for _, f := range files {
		if !m.showHidden && strings.HasPrefix(f.Name(), ".") {
			continue
		}
		filteredFiles = append(filteredFiles, f)
	}

	// Add parent directory option
	absPath, _ := filepath.Abs(m.currentDir)
	if absPath != "/" && absPath != filepath.Dir(absPath) {
		parentInfo := &setupParentDirInfo{name: ".."}
		allFiles := make([]os.FileInfo, 0, len(filteredFiles)+1)
		allFiles = append(allFiles, parentInfo)
		allFiles = append(allFiles, filteredFiles...)
		filteredFiles = allFiles
	}

	// Sort files
	sort.Slice(filteredFiles, func(i, j int) bool {
		if filteredFiles[i].Name() == ".." {
			return true
		}
		if filteredFiles[j].Name() == ".." {
			return false
		}
		
		if filteredFiles[i].IsDir() && !filteredFiles[j].IsDir() {
			return true
		}
		if !filteredFiles[i].IsDir() && filteredFiles[j].IsDir() {
			return false
		}
		return filteredFiles[i].Name() < filteredFiles[j].Name()
	})

	m.files = filteredFiles
	m.selectedIndex = 0
	m.viewportStart = 0
	return nil
}

// setupParentDirInfo for the ".." parent directory entry
type setupParentDirInfo struct {
	name string
}

func (p *setupParentDirInfo) Name() string       { return p.name }
func (p *setupParentDirInfo) Size() int64        { return 0 }
func (p *setupParentDirInfo) Mode() os.FileMode  { return os.ModeDir }
func (p *setupParentDirInfo) ModTime() time.Time { return time.Time{} }
func (p *setupParentDirInfo) IsDir() bool        { return true }
func (p *setupParentDirInfo) Sys() interface{}   { return nil }

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
			m.stage = 7 // Complete
			m.message = ""
			m.err = nil
		} else {
			m.stage = 6 // Stay in processing but show error
			m.message = fmt.Sprintf("Setup failed: %v", msg.err)
			m.err = msg.err
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.handleEnterKey()
		case "up", "k":
			return m.handleUpKey()
		case "down", "j":
			return m.handleDownKey()
		case "tab":
			return m.handleTabKey()
		case "h":
			return m.handleHomeKey()
		case "ctrl+h":
			return m.handleToggleHiddenKey()
		case "esc":
			return m.handleEscKey()
		case "1":
			if m.stage == 1 || m.stage == 4 { // Choice screens
				m.inputMode = 0
				return m, nil
			}
		case "2":
			if m.stage == 1 || m.stage == 4 { // Choice screens
				m.inputMode = 1
				return m, nil
			}
		}
	}

	// Handle textinput updates when in text input mode
	if (m.stage == 3 || m.stage == 5) && m.inputMode == 0 {
		var cmd tea.Cmd
		inputIndex := 0
		if m.configStep == 1 {
			inputIndex = 1
		}
		m.inputs[inputIndex], cmd = m.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

// Handler methods for different key actions
func (m *SetupModel) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.stage {
	case 0: // Info screen
		m.stage = 1 // Go to choice mode
		return m, nil
	case 1: // Production config choice
		if m.inputMode == 0 {
			m.stage = 3 // Text input
			m.inputs[0].Focus()
		} else {
			m.stage = 2 // File browser
			m.loadDirectory()
		}
		return m, nil
	case 2: // File browser for production
		return m.handleFileBrowserEnter()
	case 3: // Text input for production
		path := strings.TrimSpace(m.inputs[0].Value())
		if path == "" {
			m.message = "Please enter the production config file path"
			return m, nil
		}
		if !strings.HasSuffix(path, ".conf") {
			m.message = "Please select a .conf file"
			return m, nil
		}
		m.prodPath = path
		m.configStep = 1 // Move to nonprod
		m.stage = 4 // Choice for nonprod
		m.inputMode = 0 // Reset to text input
		return m, nil
	case 4: // Non-production config choice
		if m.inputMode == 0 {
			m.stage = 5 // Text input
			m.inputs[1].Focus()
		} else {
			m.stage = 2 // File browser (reuse)
			m.loadDirectory()
		}
		return m, nil
	case 5: // Text input for nonprod
		path := strings.TrimSpace(m.inputs[1].Value())
		if path == "" {
			m.message = "Please enter the non-production config file path"
			return m, nil
		}
		if !strings.HasSuffix(path, ".conf") {
			m.message = "Please select a .conf file"
			return m, nil
		}
		m.nonprodPath = path
		// Exit TUI and run setup, then return to main app
		return m, m.exitAndRunSetup()
	}
	return m, nil
}

func (m *SetupModel) handleFileBrowserEnter() (tea.Model, tea.Cmd) {
	if len(m.files) > 0 && m.selectedIndex < len(m.files) {
		selectedFile := m.files[m.selectedIndex]
		if selectedFile.IsDir() {
			if selectedFile.Name() == ".." {
				m.currentDir = filepath.Dir(m.currentDir)
			} else {
				m.currentDir = filepath.Join(m.currentDir, selectedFile.Name())
			}
			m.loadDirectory()
			return m, nil
		} else {
			// Select file
			filePath := filepath.Join(m.currentDir, selectedFile.Name())
			if strings.HasSuffix(strings.ToLower(selectedFile.Name()), ".conf") {
				if m.configStep == 0 {
					m.prodPath = filePath
					m.configStep = 1
					m.stage = 4 // Choice for nonprod
					m.inputMode = 0
				} else {
					m.nonprodPath = filePath
					// Exit TUI and run setup, then return to main app
					return m, m.exitAndRunSetup()
				}
				return m, nil
			} else {
				m.message = "Please select a .conf file"
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *SetupModel) handleUpKey() (tea.Model, tea.Cmd) {
	if m.stage == 2 && len(m.files) > 0 { // File browser
		if m.selectedIndex > 0 {
			m.selectedIndex--
			if m.selectedIndex < m.viewportStart {
				m.viewportStart = m.selectedIndex
			}
		}
	} else if m.stage == 1 || m.stage == 4 { // Choice screens
		m.inputMode = 1 - m.inputMode // Toggle
	}
	return m, nil
}

func (m *SetupModel) handleDownKey() (tea.Model, tea.Cmd) {
	if m.stage == 2 && len(m.files) > 0 { // File browser
		if m.selectedIndex < len(m.files)-1 {
			m.selectedIndex++
			if m.selectedIndex >= m.viewportStart+m.viewportSize {
				m.viewportStart = m.selectedIndex - m.viewportSize + 1
			}
		}
	} else if m.stage == 1 || m.stage == 4 { // Choice screens
		m.inputMode = 1 - m.inputMode // Toggle
	}
	return m, nil
}

func (m *SetupModel) handleTabKey() (tea.Model, tea.Cmd) {
	if m.stage == 1 || m.stage == 4 { // Choice screens
		m.inputMode = 1 - m.inputMode // Toggle
	}
	return m, nil
}

func (m *SetupModel) handleHomeKey() (tea.Model, tea.Cmd) {
	if m.stage == 2 { // File browser
		homeDir := os.Getenv("HOME")
		if homeDir != "" {
			m.currentDir = homeDir
			m.loadDirectory()
		}
	}
	return m, nil
}

func (m *SetupModel) handleToggleHiddenKey() (tea.Model, tea.Cmd) {
	if m.stage == 2 { // File browser
		m.showHidden = !m.showHidden
		m.loadDirectory()
	}
	return m, nil
}

func (m *SetupModel) handleEscKey() (tea.Model, tea.Cmd) {
	switch m.stage {
	case 1: // Choice -> Info
		m.stage = 0
		m.message = ""
	case 2, 3: // File browser or text input -> Choice
		if m.configStep == 0 {
			m.stage = 1
		} else {
			m.stage = 4
		}
		m.message = ""
	case 4: // Nonprod choice -> back to prod (if we want to change prod selection)
		m.stage = 1
		m.configStep = 0
		m.prodPath = ""
		m.message = ""
	case 5: // Nonprod text input -> Choice
		m.stage = 4
		m.message = ""
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

	case 1: // Production config choice
		s.WriteString("Step 1: Production Configuration\n\n")
		s.WriteString("Choose how to select your production config file:\n\n")
		
		if m.inputMode == 0 {
			s.WriteString("> 1. Type file path manually\n")
			s.WriteString("  2. Browse files\n")
		} else {
			s.WriteString("  1. Type file path manually\n")
			s.WriteString("> 2. Browse files\n")
		}
		
		s.WriteString("\nUse ↑/↓ or Tab to switch, Enter to select, Esc to go back")

	case 2: // File browser
		return m.buildFileBrowserView()

	case 3: // Text input for production
		s.WriteString("Step 1: Production Configuration\n\n")
		s.WriteString("Enter the path to your production WireGuard config file:\n")
		s.WriteString("(This should contain your production private key and settings)\n\n")
		s.WriteString(m.inputs[0].View())
		s.WriteString("\n\nPress Enter to confirm, Esc to go back")

	case 4: // Non-production config choice
		s.WriteString("Step 2: Non-Production Configuration\n\n")
		s.WriteString(fmt.Sprintf("Production config: %s\n\n", m.prodPath))
		s.WriteString("Choose how to select your non-production config file:\n\n")
		
		if m.inputMode == 0 {
			s.WriteString("> 1. Type file path manually\n")
			s.WriteString("  2. Browse files\n")
		} else {
			s.WriteString("  1. Type file path manually\n")
			s.WriteString("> 2. Browse files\n")
		}
		
		s.WriteString("\nUse ↑/↓ or Tab to switch, Enter to select, Esc to change production config")

	case 5: // Text input for nonprod
		s.WriteString("Step 2: Non-Production Configuration\n\n")
		s.WriteString(fmt.Sprintf("Production config: %s\n\n", m.prodPath))
		s.WriteString("Enter the path to your non-production WireGuard config file:\n")
		s.WriteString("(This should contain your non-production private key and settings)\n\n")
		s.WriteString(m.inputs[1].View())
		s.WriteString("\n\nPress Enter to start setup, Esc to go back")

	case 6: // Processing
		s.WriteString("Processing configuration files...\n\n")
		s.WriteString("This requires sudo privileges to write to /etc/wireguard/\n")

	case 7: // Complete
		s.WriteString(setupSuccessStyle.Render("Configuration Paths Selected!"))
		s.WriteString("\n\n")
		s.WriteString("Configuration file paths have been saved:\n")
		s.WriteString(fmt.Sprintf("• Production: %s\n", m.prodPath))
		s.WriteString(fmt.Sprintf("• Non-Production: %s\n", m.nonprodPath))
		s.WriteString("\n")
		s.WriteString("You can now proceed to the main application to:\n")
		s.WriteString("• Start VPN connections\n")
		s.WriteString("• Update configurations as needed\n\n")
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

func (m *SetupModel) buildFileBrowserView() string {
	var s strings.Builder
	
	configType := "Production"
	if m.configStep == 1 {
		configType = "Non-Production"
	}
	
	s.WriteString(fmt.Sprintf("Step %d: %s Configuration\n\n", m.configStep+1, configType))
	s.WriteString("Browse for your WireGuard config file:\n")
	
	hiddenStatus := "Hidden files: OFF"
	if m.showHidden {
		hiddenStatus = "Hidden files: ON"
	}
	s.WriteString(fmt.Sprintf("📁 Current directory: %s | %s\n", m.currentDir, hiddenStatus))
	s.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	s.WriteString("📂 = Directory | 📄 = File | ↑↓ Navigate | → Enter directory | Enter = Select .conf file\n")
	s.WriteString("Shortcuts: h = Home | Ctrl+H = Toggle hidden files | Esc = Go back\n")
	s.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	
	// Display files
	viewportEnd := m.viewportStart + m.viewportSize
	if viewportEnd > len(m.files) {
		viewportEnd = len(m.files)
	}
	
	if m.viewportStart > 0 {
		s.WriteString("  ↑ (more files above)\n")
	}
	
	for i := m.viewportStart; i < viewportEnd; i++ {
		file := m.files[i]
		cursor := "  "
		if i == m.selectedIndex {
			cursor = "> "
		}
		
		icon := "📄"
		if file.IsDir() {
			icon = "📂"
		}
		
		name := file.Name()
		if file.IsDir() {
			name += "/"
		}
		
		s.WriteString(fmt.Sprintf("%s%s %s\n", cursor, icon, name))
	}
	
	if viewportEnd < len(m.files) {
		s.WriteString("  ↓ (more files below)\n")
	}
	
	if len(m.files) == 0 {
		s.WriteString("(No files found in this directory)\n")
	}
	
	return s.String()
}

func (m *SetupModel) exitAndRunSetup() tea.Cmd {
	return func() tea.Msg {
		return ExitAndSetupMsg{
			prodPath:    m.prodPath,
			nonprodPath: m.nonprodPath,
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