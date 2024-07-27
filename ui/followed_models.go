package ui

import (
	"strings"
    "fmt"
    //"go-fansly-scraper/config"
    //"go-fansly-scraper/core"
    

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/table"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleFollowedModelsMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
				m.quit = true
				return m, tea.Quit
            case key.Matches(msg, m.keys.Reset):
                m.filteredModels = m.followedModels // Reset to unfiltered list
                m.filterInput = "" // Reset filter input
                m.updateTable() // Update table to show unfiltered list
                return m, nil
		    case key.Matches(msg, m.keys.Up):
                m.table.MoveUp(1)
				return m, nil
			case key.Matches(msg, m.keys.Down):
                m.table.MoveDown(1)
				return m, nil	
			case key.Matches(msg, m.keys.Select):
                selectedRow := m.table.SelectedRow()
				m.selectedModel = selectedRow[0]
                m.filteredModels = m.followedModels // Reset to unfiltered list
                m.filterInput = "" // Reset filter input
                m.updateTable()
                for _, model := range m.followedModels {
                    if model.Username == m.selectedModel {
                        m.selectedModelId = model.ID
                        break
                    }
                }
                return m.handleFollowedModelsSelection()
				// Handle post-download or other actions for the selected model here
            case key.Matches(msg, m.keys.Filter):
                m.state = FilterState
                return m, nil
			case key.Matches(msg, m.keys.Back):
				m.state = MainMenuState
                m.cursorPos = 0
				return m, nil
		}
	}
	return m, nil
}

// handleMainMenuSelection processes the selected option in the main menu
func (m *MainModel) handleFollowedModelsSelection() (tea.Model, tea.Cmd) {
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

// RenderMainMenu renders the main menu view
func (m *MainModel) RenderFollowedModelsMenu() string {
	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render(m.welcome) + "\n")
        switch m.actionChosen {
        case "download":
            sb.WriteString("Who would you like to scrape? \n")
        case "monitor":
             sb.WriteString("Who would you like to monitor? \n")
        case "like":
             sb.WriteString("Who do you want to like all post from? \n")
        case "unlike":
             sb.WriteString("Who do you want to unlike all post from? \n")
        }
    sb.WriteString(m.table.View() + "\n")
    helpView := m.help.View(m.keys)
    height := m.height - strings.Count(helpView, "\n") - m.table.Height() - 8 

    sb.WriteString("\n" + strings.Repeat("\n", height) + helpView)

	return sb.String()
}

func (m *MainModel) updateTable() {
        columns := []table.Column{
            {Title: "Username", Width: 20},
            //{Title: "AccountId", Width: 20},
            {Title: "Images", Width: 10},
            {Title: "Videos", Width: 10},
            {Title: "Bundles", Width: 10},
            {Title: "Bundle Images", Width: 15},
            {Title: "Bundle Videos", Width: 15},
        }

        rows := make([]table.Row, len(m.filteredModels))
        for i, model := range m.filteredModels {
            rows[i] = table.Row{
                model.Username,
                //model.ID,
                fmt.Sprintf("%d", model.TimelineStats.ImageCount),
                fmt.Sprintf("%d", model.TimelineStats.VideoCount),
                fmt.Sprintf("%d", model.TimelineStats.BundleCount),
                fmt.Sprintf("%d", model.TimelineStats.BundleImgCount),
                fmt.Sprintf("%d", model.TimelineStats.BundleVidCount),
            }
        }

        tableHeight := m.height - 10

        t := table.New(
            table.WithColumns(columns),
            table.WithRows(rows),
            table.WithFocused(true),
            table.WithHeight(tableHeight),
        )

        s := table.DefaultStyles()
        s.Header = s.Header.
            BorderStyle(lipgloss.NormalBorder()).
            BorderForeground(lipgloss.Color("240")).
            BorderBottom(true).
            Bold(false)
        s.Selected = s.Selected.
            Foreground(lipgloss.Color("229")).
            Background(lipgloss.Color("#cba6f7")).
            Bold(false)
        t.SetStyles(s)

        m.table = t
    
}
