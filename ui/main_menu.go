package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	//"log"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/updater"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleMainMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		logger.Logger.Printf("Received update check message: available=%v, version=%s\n", msg.Available, msg.Version)
		m.UpdateAvailable = msg.Available
		m.LatestVersion = msg.Version
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			m.cursorPos = (m.cursorPos - 1 + len(m.options)) % len(m.options)
		case key.Matches(msg, m.keys.Down):
			m.cursorPos = (m.cursorPos + 1) % len(m.options)
		case key.Matches(msg, m.keys.Select):
			m.selected = m.options[m.cursorPos]
			return m.handleMainMenuSelection()
		}
	}
	return m, nil
}

// handleMainMenuSelection processes the selected option in the main menu
func (m *MainModel) handleMainMenuSelection() (tea.Model, tea.Cmd) {
	switch m.selected {
	case "Download a user's post":
		m.actionChosen = "download"
		if !m.accountsFetched {
			m.state = LoadingState
			m.loadingMessage = "Fetching your followed accounts..."
			m.isLoading = true
			m.loadingDots = 0
			return m, tea.Batch(
				m.fetchAccountInfoCmd(),
				loadingTickCmd(),
			)
		} else {
			m.state = FollowedModelsState
			m.updateTable()
			return m, nil
		}
	case "Download purchased content":
		m.actionChosen = "download_purchases"
		m.state = DownloadPurchasedState
		return m, func() tea.Msg {
			err := m.downloader.DownloadPurchasedContent(context.Background())
			if err != nil {
				logger.Logger.Printf("Error downloading purchased content: %v", err)
				return downloadErrorMsg{Error: err}
			}
			return downloadCompleteMsg{}
		}
		//return m, m.fetchAccountInfoCmd()
	case "Monitor a user's livestreams":
		m.actionChosen = "monitor"
		if !m.accountsFetched {
			m.state = LoadingState
			m.loadingMessage = "Fetching your followed accounts..."
			m.isLoading = true
			m.loadingDots = 0
			return m, tea.Batch(
				m.fetchAccountInfoCmd(),
				loadingTickCmd(),
			)
		} else {
			m.state = LiveMonitorState
			m.filteredLiveMonitorModels = m.followedModels
			m.updateMonitoringTable()
			return m, nil
		}
	case "Like all of a user's post":
		m.actionChosen = "like"
		if !m.accountsFetched {
			m.state = LoadingState
			m.loadingMessage = "Fetching your followed accounts..."
			m.isLoading = true
			m.loadingDots = 0
			return m, tea.Batch(
				m.fetchAccountInfoCmd(),
				loadingTickCmd(),
			)
		} else {
			m.state = FollowedModelsState
			m.updateTable()
			return m, nil
		}
	case "Unlike all of a user's post":
		m.actionChosen = "unlike"
		if !m.accountsFetched {
			m.state = LoadingState
			m.loadingMessage = "Fetching your followed accounts..."
			m.isLoading = true
			m.loadingDots = 0
			return m, tea.Batch(
				m.fetchAccountInfoCmd(),
				loadingTickCmd(),
			)
		} else {
			m.state = FollowedModelsState
			m.updateTable()
			return m, nil
		}
	case "Edit config.toml file":
		configPath := config.GetConfigPath()
		err := config.EnsureConfigExists(configPath)
		if err != nil {
			logger.Logger.Printf("Error ensuring config exists: %v", err)
			return m, nil
		}
		return m, tea.ExecProcess(exec.Command(m.getEditor(), configPath), func(err error) tea.Msg {
			if err != nil {
				logger.Logger.Printf("Error editing config: %v", err)
			}
			return editConfigFinishedMsg{}
		})
	case "Run setup wizard":
		return m, func() tea.Msg {
			p := tea.NewProgram(NewConfigWizardModel())
			if _, err := p.Run(); err != nil {
				logger.Logger.Printf("Error running setup wizard: %v", err)
			}
			return setupWizardFinishedMsg{}
		}
	case "Reset configuration":
		return m, func() tea.Msg {
			if err := config.ResetConfig(); err != nil {
				logger.Logger.Printf("Error resetting config: %v", err)
				return setupWizardFinishedMsg{}
			}
			logger.Logger.Printf("Configuration reset to defaults. Launching setup wizard...")
			p := tea.NewProgram(NewConfigWizardModel())
			if _, err := p.Run(); err != nil {
				logger.Logger.Printf("Error running setup wizard after reset: %v", err)
			}
			return setupWizardFinishedMsg{}
		}
	case "Quit":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

// RenderMainMenu renders the main menu view
func (m *MainModel) RenderMainMenu() string {
	var sb strings.Builder

	// Initial Load Welcome message
	configPath := config.GetConfigPath()
	styledConfigPath := lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render(configPath)
	welcomeMessage := "Config path: " + styledConfigPath + "\n" + "Welcome to Fansly-scraper Version " + m.version
	if m.UpdateAvailable {
		updateMsg := fmt.Sprintf(" (Update %s available)", m.LatestVersion)
		welcomeMessage += lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Render(updateMsg)
	}
	styledWelcomeMessage := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Render(welcomeMessage)
	sb.WriteString(styledWelcomeMessage + "\n")

	// Maintainer Repo
	repoLink := "https://github.com/agnosto/fansly-scraper"
	styledRepoLink := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Render(repoLink)
	sb.WriteString("Maintainer's repo: " + styledRepoLink + "\n\n")

	sb.WriteString("What would you like to do? " + "\n")

	for i, opt := range m.options {
		if i == m.cursorPos {
			sb.WriteString("> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")).Render(opt) + "\n")
		} else {
			sb.WriteString("  " + opt + "\n")
		}
	}

	return sb.String()
}

func (m *MainModel) getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vim"
		}
	}
	return editor
}

// fetchAccountInfoCmd is a command that fetches account info
func (m *MainModel) fetchAccountInfoCmd() tea.Cmd {
	return func() tea.Msg {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Create a channel for the result
		resultCh := make(chan fetchAccountInfoMsg, 1)

		// Start the fetch in a goroutine
		go func() {
			accountInfo, err := core.FetchAccountInfo(config.GetConfigPath())
			if err != nil {
				logger.Logger.Printf("Error fetching account info: %v", err)
				resultCh <- fetchAccountInfoMsg{Success: false, Error: err}
			} else {
				resultCh <- fetchAccountInfoMsg{Success: true, AccountInfo: accountInfo}
			}
		}()

		// Wait for either the result or timeout
		select {
		case result := <-resultCh:
			return result
		case <-ctx.Done():
			return fetchAccountInfoMsg{
				Success: false,
				Error:   fmt.Errorf("timeout fetching account information"),
			}
		}
	}
}

func (m *MainModel) checkForUpdates() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadConfig(config.GetConfigPath())
		if err != nil || !cfg.Options.CheckUpdates {
			return nil
		}

		logger.Logger.Printf("Checking for updates. Current version: %s\n", m.version)
		available, latestVer, err := updater.CheckUpdateAvailable(m.version)
		if err != nil {
			return nil
		}

		logger.Logger.Printf("Update check result: available=%v, latest=%s\n", available, latestVer)
		return updateCheckMsg{
			Available: available,
			Version:   latestVer,
		}
	}
}
