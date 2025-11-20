package ui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

func (m *MainModel) HandleCompletionUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			m.cursorPos = (m.cursorPos - 1 + 3) % 3
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.cursorPos = (m.cursorPos + 1) % 3
			return m, nil
		case key.Matches(msg, m.keys.Select):
			switch m.cursorPos {
			case 0: // Process another
				m.state = FollowedModelsState
				m.filteredModels = m.followedModels
				m.filterInput = ""
				m.updateTable()
				m.cursorPos = 0
				return m, nil
			case 1: // Return to main menu
				m.state = MainMenuState
				m.cursorPos = 0
				return m, nil
			case 2: // Quit
				m.quit = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *MainModel) RenderCompletionMenu() string {
	var sb strings.Builder
	options := []string{
		fmt.Sprintf("Process another %s", m.actionChosen),
		"Return to main menu",
		"Quit",
	}

	sb.WriteString("Operation completed successfully!\n\n")
	sb.WriteString("What would you like to do next?\n\n")

	for i, opt := range options {
		if i == m.cursorPos {
			sb.WriteString("> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")).Render(opt) + "\n")
		} else {
			sb.WriteString("  " + opt + "\n")
		}
	}

	return sb.String()
}
