package ui

import (
	"fmt"
	"strings"
	//"github.com/agnosto/fansly-scraper/config"
	//"github.com/agnosto/fansly-scraper/core"

	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/key"
	//"github.com/charmbracelet/bubbles/table"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandlePurchaseProgressMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.state = MainMenuState
			m.cursorPos = 0
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// RenderMainMenu renders the main menu view
func (m *MainModel) RenderPurchaseProgressMenu() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Downloading purchased content...\n\n"))

	return sb.String()
}
