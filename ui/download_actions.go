package ui

import (
	"strings"
    "fmt"
    "sync"
    "log"
    "context"
    //"github.com/agnosto/fansly-scraper/auth"
    //"github.com/agnosto/fansly-scraper/config"
    //"github.com/agnosto/fansly-scraper/core"
    "github.com/agnosto/fansly-scraper/logger"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/bubbles/key"
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
                m.state = DownloadProgressState
                return m, m.startDownload(m.selectedOption)
			case key.Matches(msg, m.keys.Back):
				m.state = FollowedModelsState
				m.cursorPos = 0
				return m, nil
            }
	}
	return m, nil
}

/*
func (m *MainModel) handleDownloadActionsSelection() (tea.Model, tea.Cmd) {
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
            switch option {
            case "All": 
                if err := m.downloader.DownloadTimeline(ctx, m.selectedModelId, m.selectedModel); err != nil {
                    logger.Logger.Printf("Error downloading timeline: %v", err)
                }
                if err := m.downloader.DownloadMessages(ctx, m.selectedModelId, m.selectedModel); err != nil {
                    logger.Logger.Printf("Error downloading messages: %v", err)
                }
                if err := m.downloader.DownloadStories(ctx, m.selectedModelId, m.selectedModel); err != nil {
                    logger.Logger.Printf("Error downloading messages: %v", err)
                }
            case "Timeline":
                err = m.downloader.DownloadTimeline(ctx, m.selectedModelId, m.selectedModel)
            case "Messages":
                err = m.downloader.DownloadMessages(ctx, m.selectedModelId, m.selectedModel)
            case "Stories":
                err = m.downloader.DownloadStories(ctx, m.selectedModelId, m.selectedModel)
            }
            if err != nil {
                if err.Error() == "not subscribed or followed: unable to get timeline feed" {
                    log.Printf("Unable to download timeline for %s: Not subscribed or followed", m.selectedModel)
                    // You might want to update the UI here to show this message
                } else {
                    logger.Logger.Printf("Error downloading %v for %s: %v", option, m.selectedModel, err)
                    log.Printf("Error downloading %s for %s: %v", option, m.selectedModel, err)
                } 
            }
            //log.Println("Download goroutine finished")
        }()

        //m.state = MainMenuState
        //return nil
        return func() tea.Msg {
            wg.Wait()
            m.state = MainMenuState
            return downloadCompleteMsg{}
        }
    }
}
