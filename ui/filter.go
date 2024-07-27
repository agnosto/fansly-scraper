package ui

import (
	"strings"
    //"fmt"
    "go-fansly-scraper/auth"
    //"go-fansly-scraper/config"
    //"go-fansly-scraper/core"
    

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
    //"github.com/charmbracelet/bubbles/key"
    //"github.com/charmbracelet/bubbles/table"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleFilterModelsMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
            case "ctrl+c":
                m.quit = true
                return m, tea.Quit
            case "esc":
                m.filteredModels = m.followedModels // Reset to unfiltered list
                m.filterInput = "" // Reset filter input
                m.updateTable() // Update table to show unfiltered list
                m.state = FollowedModelsState
                return m, nil
            case "up":
            //    m.table.MoveUp(1)
				return m, nil
			case "down":
            //    m.table.MoveDown(1)
				return m, nil
            case "enter":
                m.applyFilter()
                m.state = FollowedModelsState
                m.filterInput = ""
                return m, nil
            case "backspace":
                if len(m.filterInput) > 0 {
						m.filterInput = m.filterInput[:len(m.filterInput)-1]
                        m.applyFilter()
				}
            default:
                m.filterInput += msg.String()
                m.applyFilter()
                return m, nil
            }
	}
	return m, nil
}

/*
func (m *MainModel) handleFilterModelsSelection() (tea.Model, tea.Cmd) {
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

func (m *MainModel) RenderFilterModelsMenu() string {
	var sb strings.Builder

    sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render(m.welcome) + "\n")
    sb.WriteString("Filter by username: " + m.filterInput + "\n")
    sb.WriteString(m.table.View() + "\n")

	return sb.String()
}

func (m *MainModel) applyFilter() {
	filtered := []auth.FollowedModel{}
	for _, model := range m.followedModels {
		if strings.Contains(strings.ToLower(model.Username), strings.ToLower(m.filterInput)) {
			filtered = append(filtered, model)
		}
	}
	m.filteredModels = filtered
	m.updateTable()
}
