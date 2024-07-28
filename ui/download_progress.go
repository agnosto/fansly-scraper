package ui

import (
	"strings"
    "fmt"
    //"github.com/agnosto/fansly-scraper/config"
    //"github.com/agnosto/fansly-scraper/core"
    

	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/bubbles/key"
    //"github.com/charmbracelet/bubbles/table"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleDownloadProgressMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
        case key.Matches(msg, m.keys.Back):
            m.state = FollowedModelsState
            m.cursorPos = 0
            return m, nil
        case key.Matches(msg, m.keys.Quit):
            m.quit = true
            return m, tea.Quit
        }
	}
	return m, nil
}

/*
func (m *MainModel) handleDownloadProgressSelection() (tea.Model, tea.Cmd) {
	switch m.actionChosen {
        case "download":
            m.state = DownloadActionsState
        case "monitor":
            m.state = LiveMonitorState
        case "like":
            m.state = LikePostState
        case "unlike":
            m.state = UnlikePostState
        }
	return m, nil
}
*/

// RenderMainMenu renders the main menu view
func (m *MainModel) RenderDownloadProgressMenu() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Downloading %s content for %s...\n\n", m.selectedOption, m.selectedModel))


	return sb.String()
}
