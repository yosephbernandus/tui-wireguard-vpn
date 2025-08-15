package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type model struct {
	title   string
	status  string
	choices []string
	cursor  int
}

func initialModel() model {
	return model{
		title:  "WireGuard VPN Manager",
		status: "Disconnected",
		choices: []string{
			"Start Production VPN",
			"Start Non-Production VPN", 
			"Stop VPN",
			"Show Status",
			"Update Configuration",
			"Quit",
		},
		cursor: 0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
			case 0:
				m.status = "Starting Production VPN..."
			case 1:
				m.status = "Starting Non-Production VPN..."
			case 2:
				m.status = "Stopping VPN..."
			case 3:
				m.status = "Checking VPN status..."
			case 4:
				m.status = "Updating configuration..."
			case 5:
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render(m.title) + "\n\n"
	s += statusStyle.Render("Status: "+m.status) + "\n\n"

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\n" + helpStyle.Render("Use ↑/↓ or j/k to navigate, Enter to select, q to quit")
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}