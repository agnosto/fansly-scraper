package ui

import (
	"context"
	"fmt"

	//"log"
	"strings"
	"sync"

	//"github.com/agnosto/fansly-scraper/auth"
	//"github.com/agnosto/fansly-scraper/config"
	//"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/logger"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	//"github.com/charmbracelet/bubbles/table"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleDownloadActionsMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			m.cursorPos = (m.cursorPos - 1 + len(m.downloadOptions)) % len(m.downloadOptions)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.cursorPos = (m.cursorPos + 1) % len(m.downloadOptions)
			return m, nil
		case key.Matches(msg, m.keys.Select):
			m.selectedOption = m.downloadOptions[m.cursorPos]
			if m.selectedOption == "Select Wall" {
				m.state = WallSelectionState
				m.cursorPos = 0 // Reset cursor for the new menu
				m.updateWallTable()
				return m, nil
			}
			m.state = DownloadProgressState
			return m, m.startDownload(m.selectedOption)
		case key.Matches(msg, m.keys.Back):
			m.state = FollowedModelsState
			m.cursorPos = 0
			m.updateTable()
			return m, nil
		}
	}
	return m, nil
}

func (m *MainModel) RenderDownloadActionsMenu() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("What would you like to scrape from %s?\n", m.selectedModel))
	for i, opt := range m.downloadOptions {
		if i == m.cursorPos {
			sb.WriteString("> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")).Render(opt) + "\n")
		} else {
			sb.WriteString("  " + opt + "\n")
		}
	}
	helpView := m.help.View(m.keys)
	sb.WriteString("\n" + helpView)

	return sb.String()
}

func (m *MainModel) startDownload(option string) tea.Cmd {
	return func() tea.Msg {
		//log.Println("startDownload function called")
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			//log.Println("Starting download goroutine")
			defer wg.Done()
			var err error
			ctx := context.Background()

			var targetModel auth.FollowedModel
			for _, mod := range m.followedModels {
				if mod.ID == m.selectedModelId {
					targetModel = mod
					break
				}
			}
			// Fallback: Check filtered models if not found
			if targetModel.ID == "" {
				for _, mod := range m.filteredModels {
					if mod.ID == m.selectedModelId {
						targetModel = mod
						break
					}
				}
			}

			switch option {
			case "All":
				if err := m.downloader.DownloadTimeline(ctx, m.selectedModelId, m.selectedModel, ""); err != nil {
					logger.Logger.Printf("Error downloading timeline: %v", err)
				}
				if err := m.downloader.DownloadMessages(ctx, m.selectedModelId, m.selectedModel); err != nil {
					logger.Logger.Printf("Error downloading messages: %v", err)
				}
				if err := m.downloader.DownloadStories(ctx, m.selectedModelId, m.selectedModel); err != nil {
					logger.Logger.Printf("Error downloading messages: %v", err)
				}
				if targetModel.ID != "" {
					if err := m.downloader.DownloadProfileContent(ctx, targetModel); err != nil {
						logger.Logger.Printf("Error downloading profile content: %v", err)
					}
				}
			case "Timeline":
				err = m.downloader.DownloadTimeline(ctx, m.selectedModelId, m.selectedModel, "")
			case "Messages":
				err = m.downloader.DownloadMessages(ctx, m.selectedModelId, m.selectedModel)
			case "Stories":
				err = m.downloader.DownloadStories(ctx, m.selectedModelId, m.selectedModel)
			}

			if err != nil {
				logger.Logger.Printf("Error downloading %s for %s: %v", option, m.selectedModel, err)
			}
			//log.Println("Download goroutine finished")
		}()

		//return func() tea.Msg {
		wg.Wait()
		//m.state = MainMenuState
		return downloadCompleteMsg{}
		//}
	}
}
