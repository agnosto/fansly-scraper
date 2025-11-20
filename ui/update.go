package ui

import (
	//"strings"
	//"fmt"
	//"github.com/agnosto/fansly-scraper/auth"
	//"github.com/agnosto/fansly-scraper/config"
	//"github.com/agnosto/fansly-scraper/core"
	"time"

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
		case key.Matches(msg, m.keys.Refresh):
			if m.state == FollowedModelsState || m.state == LiveMonitorState {
				m.state = LoadingState
				m.loadingMessage = "Refreshing your followed accounts..."
				m.isLoading = true
				m.loadingDots = 0
				m.accountsFetched = false // Reset the flag to force a refresh
				return m, tea.Batch(
					m.fetchAccountInfoCmd(),
					loadingTickCmd(),
				)
			}
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
		case LiveMonitorState:
			return m.HandleLivestreamMonitorUpdate(msg)
		case LiveMonitorFilterState:
			return m.HandleLiveMonitorFilterUpdate(msg)
		case DownloadPurchasedState:
			return m.HandlePurchaseProgressMenuUpdate(msg)
		case CompletionState:
			return m.HandleCompletionUpdate(msg)
		case DonateState:
			return m.HandleDonateMenuUpdate(msg)
		case WallSelectionState:
			return m.HandleWallSelectionUpdate(msg)
		// Add cases for other states
		default:
			logger.Logger.Printf("[DEBUG] Update: Unhandled state: %v", m.state)
			return m, nil
		}
	case loadingTickMsg:
		if m.state == LoadingState {
			m.loadingDots = (m.loadingDots + 1) % 4
			return m, loadingTickCmd()
		}
		return m, nil
	case fetchAccountInfoMsg:
		m.isLoading = false
		if msg.Success {
			m.welcome = msg.AccountInfo.Welcome
			m.followedModels = msg.AccountInfo.FollowedModels
			m.filteredModels = msg.AccountInfo.FollowedModels
			m.accountsFetched = true // Set the flag when accounts are successfully fetched

			if m.actionChosen == "monitor" {
				m.state = LiveMonitorState
				m.filteredLiveMonitorModels = m.followedModels
				m.initializeLivestreamMonitoringTable()
				m.updateMonitoringTable()
			} else {
				m.state = FollowedModelsState
				m.updateTable()
			}
		} else {
			// Handle error
			m.state = MainMenuState
			m.message = "Error fetching account info: " + msg.Error.Error()
		}
		return m, nil
	case monitoringSelectedMsg:
		m.state = LiveMonitorState
		m.filteredLiveMonitorModels = m.followedModels
		m.updateMonitoringTable()
	case editConfigFinishedMsg:
		// Config editing is finished, refresh the UI
		return m, tea.ClearScreen
	case downloadCompleteMsg:
		return m, tea.Sequence(
			tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return delayedDownloadCompleteMsg{}
			}),
		)
	case delayedDownloadCompleteMsg:
		m.state = CompletionState
		m.cursorPos = 0
		return m, nil
	case likeUnlikeCompletedMsg:
		return m, tea.Sequence(
			tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return delayedLikeUnlikeCompleteMsg{}
			}),
		)
	case delayedLikeUnlikeCompleteMsg:
		m.state = CompletionState
		m.cursorPos = 0
		return m, nil
	case LiveStatusUpdateMsg:
		if m.state == LiveMonitorState {
			m.updateMonitoringTable()
			return m, tea.Tick(2*time.Minute, func(t time.Time) tea.Msg {
				return LiveStatusUpdateMsg{}
			})
		}
		return m, nil
	case editConfigMsg:
		if msg.Success {
			m.message = "Config edited successfully!"
			m.accountsFetched = false
			//return m, tea.ClearScreen
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
	case LoadingState:
		return m.RenderLoadingScreen()
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
	case LiveMonitorState:
		return m.RenderLivestreamMonitorMenu()
	case LiveMonitorFilterState:
		return m.RenderLiveMonitorFilterMenu()
	case DownloadPurchasedState:
		return m.RenderPurchaseProgressMenu()
	case CompletionState:
		return m.RenderCompletionMenu()
	case DonateState:
		return m.RenderDonateMenu()
	case WallSelectionState:
		return m.RenderWallSelectionMenu()
	default:
		return "Unknown state"
	}
}
