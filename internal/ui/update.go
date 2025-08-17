package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	filepicker    filepicker.Model
	textinput     textinput.Model
	stage         int // 0: info, 1: choose mode, 2: text input, 3: file picker, 4: processing, 5: complete
	inputMode     int // 0: text input, 1: file browser
	message       string
	err           error
	configPath    string
	// Custom file browser
	currentDir    string
	files         []os.FileInfo
	selectedIndex int
	showHidden    bool
	// Scrolling support
	viewportStart int // First visible item index
	viewportSize  int // Number of items visible at once
}

func NewUpdateModel() *UpdateModel {
	// Setup text input
	ti := textinput.New()
	ti.Placeholder = "/path/to/config.conf"
	ti.CharLimit = 256
	ti.Width = 50

	// Get current working directory - start from where user ran the app
	currentDir, err := os.Getwd()
	if err != nil {
		// Fallback to user's home directory if we can't get current dir
		currentDir = os.Getenv("HOME")
		if currentDir == "" {
			currentDir = "." // Last resort
		}
	}

	model := &UpdateModel{
		textinput:     ti,
		stage:         3,    // Start directly in file picker mode for panel embedding
		inputMode:     1,    // File browser mode
		currentDir:    currentDir,
		selectedIndex: 0,
		showHidden:    true,  // Show all files including hidden ones by default
		viewportStart: 0,
		viewportSize:  15,   // Show 15 files at once
	}

	// Load directory contents
	if err := model.loadDirectory(); err != nil {
		model.message = fmt.Sprintf("Error loading directory: %v", err)
	}
	
	return model
}

func (m *UpdateModel) loadDirectory() error {
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
		// Skip hidden files unless showHidden is true
		if !m.showHidden && strings.HasPrefix(f.Name(), ".") {
			continue
		}
		filteredFiles = append(filteredFiles, f)
	}

	// Add parent directory option if not in root and not already at filesystem root
	absPath, _ := filepath.Abs(m.currentDir)
	if absPath != "/" && absPath != filepath.Dir(absPath) {
		// Create a fake parent directory entry
		parentInfo := &parentDirInfo{name: ".."}
		allFiles := make([]os.FileInfo, 0, len(filteredFiles)+1)
		allFiles = append(allFiles, parentInfo)
		allFiles = append(allFiles, filteredFiles...)
		filteredFiles = allFiles
	}

	// Sort: directories first (except ..), then files alphabetically
	sort.Slice(filteredFiles, func(i, j int) bool {
		// Always keep .. at the top
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
	m.viewportStart = 0  // Reset viewport to top when loading new directory
	return nil
}

// parentDirInfo implements os.FileInfo for the ".." parent directory entry
type parentDirInfo struct {
	name string
}

func (p *parentDirInfo) Name() string       { return p.name }
func (p *parentDirInfo) Size() int64        { return 0 }
func (p *parentDirInfo) Mode() os.FileMode  { return os.ModeDir }
func (p *parentDirInfo) ModTime() time.Time { return time.Time{} }
func (p *parentDirInfo) IsDir() bool        { return true }
func (p *parentDirInfo) Sys() interface{}   { return nil }

func (m *UpdateModel) Init() tea.Cmd {
	// No initialization needed for custom file browser
	return nil
}

func (m *UpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// No special handling needed for custom file browser
		return m, nil
		
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.stage == 0 {
				return m, tea.Quit
			}
			// For panel embedding, don't quit - let main handle it
			if m.stage == 3 {
				return m, nil
			}
			// Go back to previous stage
			if m.stage > 0 {
				m.stage--
			}
			return m, nil
		case "up", "k":
			if m.stage == 3 && len(m.files) > 0 {
				if m.selectedIndex > 0 {
					m.selectedIndex--
					// Auto-scroll up if needed
					if m.selectedIndex < m.viewportStart {
						m.viewportStart = m.selectedIndex
					}
				}
				return m, nil
			}
		case "down", "j":
			if m.stage == 3 && len(m.files) > 0 {
				if m.selectedIndex < len(m.files)-1 {
					m.selectedIndex++
					// Auto-scroll down if needed
					if m.selectedIndex >= m.viewportStart+m.viewportSize {
						m.viewportStart = m.selectedIndex - m.viewportSize + 1
					}
				}
				return m, nil
			}
		case "pgup", "page_up":
			if m.stage == 3 && len(m.files) > 0 {
				// Jump up by viewport size
				m.selectedIndex -= m.viewportSize
				if m.selectedIndex < 0 {
					m.selectedIndex = 0
				}
				m.viewportStart = m.selectedIndex
				return m, nil
			}
		case "pgdown", "page_down":
			if m.stage == 3 && len(m.files) > 0 {
				// Jump down by viewport size
				m.selectedIndex += m.viewportSize
				if m.selectedIndex >= len(m.files) {
					m.selectedIndex = len(m.files) - 1
				}
				// Adjust viewport
				if m.selectedIndex >= m.viewportStart+m.viewportSize {
					m.viewportStart = m.selectedIndex - m.viewportSize + 1
				}
				return m, nil
			}
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
				return m, nil
			case 3: // Custom file browser
				if len(m.files) > 0 && m.selectedIndex < len(m.files) {
					selectedFile := m.files[m.selectedIndex]
					if selectedFile.IsDir() {
						// Handle parent directory navigation
						if selectedFile.Name() == ".." {
							// Go to parent directory
							parentDir := filepath.Dir(m.currentDir)
							m.currentDir = parentDir
							m.loadDirectory()
							return m, nil
						} else {
							// Enter subdirectory
							newDir := filepath.Join(m.currentDir, selectedFile.Name())
							m.currentDir = newDir
							m.loadDirectory()
							return m, nil
						}
					} else {
						// Select file
						filePath := filepath.Join(m.currentDir, selectedFile.Name())
						if strings.HasSuffix(strings.ToLower(selectedFile.Name()), ".conf") {
							m.configPath = filePath
							return m, nil
						} else {
							m.message = "Please select a .conf file"
							return m, nil
						}
					}
				}
			}
		case "esc":
			// For panel embedding in stage 3, don't handle esc - let main handle it
			if m.stage == 3 {
				return m, nil
			}
			if m.stage > 0 {
				m.stage--
				m.message = ""
				return m, nil
			} else {
				return m, tea.Quit
			}
		case "h":
			// Go to home directory
			if m.stage == 3 {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					m.currentDir = homeDir
					m.loadDirectory()
				}
				return m, nil
			}
		case "ctrl+h":
			// Toggle hidden files
			if m.stage == 3 {
				m.showHidden = !m.showHidden
				m.loadDirectory()
				return m, nil
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
		}
	}

	// Handle text input updates when in text input mode
	if m.stage == 2 {
		var cmd tea.Cmd
		m.textinput, cmd = m.textinput.Update(msg)
		return m, cmd
	}

	// Custom file browser is handled above in key handling

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

	case 3: // Custom file browser
		s.WriteString("Browse for your WireGuard config file:\n")
		hiddenStatus := "Hidden files: OFF"
		if m.showHidden {
			hiddenStatus = "Hidden files: ON"
		}
		s.WriteString(fmt.Sprintf("📁 Current directory: %s | %s | Files found: %d\n", m.currentDir, hiddenStatus, len(m.files)))
		s.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		s.WriteString("📂 = Directory | 📄 = File | ⬆️ ⬇️ Navigate | ➡️ Enter directory | Enter = Select .conf file\n")
		s.WriteString("Shortcuts: h = Home | Ctrl+H = Toggle hidden files | Esc = Go back\n")
		s.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
		
		// Display files and directories (viewport only)
		viewportEnd := m.viewportStart + m.viewportSize
		if viewportEnd > len(m.files) {
			viewportEnd = len(m.files)
		}
		
		// Show scrolling indicator if needed
		if m.viewportStart > 0 {
			s.WriteString("  ⬆️ (more files above)\n")
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
			
			// Highlight selected item
			line := fmt.Sprintf("%s%s %s", cursor, icon, name)
			if i == m.selectedIndex {
				// Add some visual highlight for selected item
				line = fmt.Sprintf("→ %s %s", icon, name)
			}
			s.WriteString(line + "\n")
		}
		
		// Show scrolling indicator if there are more files below
		if viewportEnd < len(m.files) {
			s.WriteString("  ⬇️ (more files below)\n")
		}
		
		if len(m.files) == 0 {
			s.WriteString("(No files found in this directory)\n")
		}
		
		s.WriteString("\n💡 Tip: Navigate with ↑↓, Enter to select/enter directories")
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