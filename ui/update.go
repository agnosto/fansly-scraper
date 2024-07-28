package ui

import (
	//"strings"
	//"fmt"
	//"github.com/agnosto/fansly-scraper/auth"
	//"github.com/agnosto/fansly-scraper/config"
	//"github.com/agnosto/fansly-scraper/core"
    "github.com/agnosto/fansly-scraper/logger"

	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/key"
	//"github.com/charmbracelet/bubbles/table"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        logger.Logger.Printf("Window size changed to %dx%d", msg.Width, msg.Height)
        m.help.Width = msg.Width
        m.width = msg.Width
        m.height = msg.Height
        m.updateTable()
		//m.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Help):
            m.help.ShowAll = !m.help.ShowAll
        case key.Matches(msg, m.keys.Quit):
            m.quit = true
			return m, tea.Quit
		}
        switch m.state {
            case MainMenuState:
                return m.HandleMainMenuUpdate(msg)
            case FollowedModelsState:
                return m.HandleFollowedModelsMenuUpdate(msg)
            case FilterState:
                return m.HandleFilterModelsMenuUpdate(msg)
            case DownloadActionsState:
                return m.HandleDownloadActionsMenuUpdate(msg)
            case DownloadProgressState:
                return m.HandleDownloadProgressMenuUpdate(msg)
            case LikeUnlikeState:
                logger.Logger.Printf("[DEBUG] Update: Handling LikeUnlikeState")
                return m.HandleLikeUnlikeUpdate(msg)
            // Add cases for other states
            default:
                logger.Logger.Printf("[DEBUG] Update: Unhandled state: %v", m.state)
                return m, nil
        } 
    case fetchAccountInfoMsg:
        if msg.Success {
            m.welcome = msg.AccountInfo.Welcome
            m.followedModels = msg.AccountInfo.FollowedModels
            m.filteredModels = msg.AccountInfo.FollowedModels
            m.updateTable()
            m.state = FollowedModelsState
        } else {
            // handle the error
        }
        return m, nil
    case editConfigMsg:
        if msg.Success {
            m.message = "Config edited successfully!"
        } else {
            m.message = "Error editing config: " + msg.Error.Error()
        }
        m.state = MainMenuState
        return m, nil
	}
	return m, nil
}

func (m *MainModel) View() string {
	switch m.state {
	case MainMenuState:
		return m.RenderMainMenu()
	// Add cases for other states
    case FollowedModelsState:
        return m.RenderFollowedModelsMenu()
    case FilterState:
        return m.RenderFilterModelsMenu()
    case DownloadActionsState:
        return m.RenderDownloadActionsMenu()
    case DownloadProgressState:
        return m.RenderDownloadProgressMenu()
    case LikeUnlikeState:
        return m.RenderLikeUnlikeMenu()
	default:
		return "Unknown state"
	}
}

