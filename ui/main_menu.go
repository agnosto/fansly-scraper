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
		return m, m.fetchAccountInfoCmd()
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
		return m, tea.Batch(
			m.fetchAccountInfoCmd(),
			func() tea.Msg {
				m.updateMonitoringTable()
				return monitoringSelectedMsg{}
			},
		)
	case "Like all of a user's post":
		m.actionChosen = "like"
		return m, m.fetchAccountInfoCmd()
	case "Unlike all of a user's post":
		m.actionChosen = "unlike"
		return m, m.fetchAccountInfoCmd()
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
		accountInfo, err := core.FetchAccountInfo(config.GetConfigPath())
		if err != nil {
			logger.Logger.Printf("Error fetching account info: %v", err)
			return fetchAccountInfoMsg{Success: false, Error: err}
		}
		return fetchAccountInfoMsg{Success: true, AccountInfo: accountInfo}
	}
}

// editConfigCmd is a command that initiates config editing
func (m *MainModel) editConfigCmd() tea.Cmd {
	return func() tea.Msg {
		configPath := config.GetConfigPath()
		err := config.EnsureConfigExists(configPath)
		if err != nil {
			logger.Logger.Printf("Error check config %v", err)
			return editConfigMsg{Success: false, Error: err}
		}

		err = config.OpenConfigInEditor(configPath)
		if err != nil {
			logger.Logger.Printf("Error opening config %v", err)
			return editConfigMsg{Success: false, Error: err}
		}

		return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return tickMsg{}
		})
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
