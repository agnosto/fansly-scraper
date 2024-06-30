package main

import (
	"fmt"
	"go-fansly-scraper/auth"
	"go-fansly-scraper/config"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/bubbles"
	"github.com/charmbracelet/lipgloss"
)

type AppState int

const (
	MainMenuState AppState = iota
	FollowedModelsState
	DownloadState
	LiveMonitorState
    LikePostState
    UnlikePostState
)

type mainModel struct {
	quit           bool
	cursorPos      int
	selected       string
	options        []string
	state          AppState
	followedModels []auth.FollowedModel
    viewportStart  int
    viewportSize   int
}

type followedModelsModel struct {
	welcome         string
	selected        string
	followedModels  []auth.FollowedModel
	cursorPos       int
}

type tickMsg struct{}

// Init implements tea.Model.
func (mainModel) Init() tea.Cmd {
	return nil
	// panic("unimplemented")
}

// Update implements tea.Model.
func (m *mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	configPath := GetConfigPath()
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case MainMenuState:
			switch msg.String() {
			case "ctrl+c", "q":
				m.quit = true
				return m, tea.Quit
			case "up":
				m.cursorPos = (m.cursorPos - 1 + len(m.options)) % len(m.options)
				return m, nil
			case "down":
				m.cursorPos = (m.cursorPos + 1) % len(m.options)
				return m, nil
			case "enter":
				m.selected = m.options[m.cursorPos]
				switch m.selected {
				case "Download a user's post":
					m.state = DownloadState

					// Load the configuration
					cfg, err := config.LoadConfig(configPath)
					if err != nil {
						log.Printf("Error loading config: %v", err)
						return m, nil
					}

					// Fetch the account information using the auth token and user agent from the config
					accountInfo, err := auth.Login(cfg.Authorization, cfg.UserAgent)
					if err != nil {
						log.Println(cfg.Authorization)
						log.Printf("Error logging in: %v", err)
						return m, nil
					}
					fmt.Println("Welcome ", accountInfo.DisplayName, " | ", accountInfo.Username)

					// Fetch the list of followed users
					followedModels, err := auth.GetFollowedUsers(accountInfo.ID, cfg.Authorization, cfg.UserAgent)
					if err != nil {
						fmt.Printf("\nError getting followed models: %v\n", err)
						return m, nil
					}
					m.followedModels = followedModels
                    m.viewportStart = 0
                    m.viewportSize = 20
					m.state = FollowedModelsState
					return m, nil
				case "Monitor a user's livestream":
					m.state = LiveMonitorState
				case "Like all of a user's post":
					m.state = LikePostState
				case "Unlike all of a user's post":
					m.state = UnlikePostState
				case "Edit config.json file":
					// Ensure the config.json file exists
					err := config.EnsureConfigExists(configPath)
					if err != nil {
						return m, nil
					}
					return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return tickMsg{}
					})
				case "Quit":
					m.quit = true
					return m, tea.Quit
				}
			}
		case FollowedModelsState:
			switch msg.String() {
			case "ctrl+c", "q":
				m.quit = true
				return m, tea.Quit
		    case "up":
				if m.cursorPos > 0 {
					m.cursorPos--
				} else {
					m.cursorPos = len(m.followedModels) - 1
				}
				if m.cursorPos < m.viewportStart {
					m.viewportStart = m.cursorPos
				} else if m.cursorPos >= m.viewportStart+m.viewportSize {
					m.viewportStart = m.cursorPos - m.viewportSize + 1
				}
				return m, nil
			case "down":
				if m.cursorPos < len(m.followedModels)-1 {
					m.cursorPos++
				} else {
					m.cursorPos = 0
				}
				if m.cursorPos >= m.viewportStart+m.viewportSize {
					m.viewportStart = m.cursorPos - m.viewportSize + 1
				} else if m.cursorPos < m.viewportStart {
					m.viewportStart = m.cursorPos
				}
				return m, nil	
			case "enter":
				selectedModel := m.followedModels[m.cursorPos]
				fmt.Printf("Selected model: %s\n", selectedModel.Username)
				// Handle post-download or other actions for the selected model here
				return m, nil
            case "/":
                return m, nil
			case "esc":
				m.state = MainMenuState
                m.cursorPos = 0
				return m, nil
			}
		}
	default:
		return m, nil
	}
	return m, nil
}

func (m *mainModel) View() string {
	var sb strings.Builder

	switch m.state {
	case MainMenuState:
		// Welcome message
		configpath := GetConfigPath()
		styledConfigPath := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC8FF")).Render(configpath)
		welcomeMessage := "Config path: " + styledConfigPath + "\n" + "Welcome to Fansly-scraper Version 0.0.1"
		styledWelcomeMessage := lipgloss.NewStyle().Foreground(lipgloss.Color("#90EE90")).Render(welcomeMessage)
		sb.WriteString(styledWelcomeMessage + "\n")
		// Maintainer Repo
		repoLink := "https://github.com/agnosto/fansly-scraper"
		styledRepoLink := lipgloss.NewStyle().Foreground(lipgloss.Color("#7676ff")).Render(repoLink)
		sb.WriteString("Maintainer's repo: " + styledRepoLink + "\n\n")

		sb.WriteString("What would you like to do? " + m.selected + "\n")

		for i, opt := range m.options {
			if i == m.cursorPos {
				sb.WriteString("> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#ADD8E6")).Render(opt) + "\n")
			} else {
				sb.WriteString("  " + opt + "\n")
			}
		}
	case FollowedModelsState:
        sb.Reset()
		sb.WriteString("Select a followed model:\n")
	    for i := m.viewportStart; i < m.viewportStart+m.viewportSize && i < len(m.followedModels); i++ {
			model := m.followedModels[i]
			if i == m.cursorPos {
				sb.WriteString("> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#ADD8E6")).Render(fmt.Sprintf("%s | images: %d | videos: %d", model.Username, model.TimelineStats.ImageCount, model.TimelineStats.VideoCount)) + "\n")
			} else {
				sb.WriteString("  " + fmt.Sprintf("%s | images: %d | videos: %d", model.Username, model.TimelineStats.ImageCount, model.TimelineStats.VideoCount) + "\n")
			}
		}	
		sb.WriteString("\nPress 'esc' to go back to the main menu.")
	}

	return sb.String()
}

func main() {
	p := tea.NewProgram(&mainModel{
		options:   []string{"Download a user's post", "Monitor a user's livestreams", "Like all of a user's post", "Unlike all of a user's post", "Edit config.json file", "Quit"},
		cursorPos: 0,
	}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	var configpath string
	switch runtime.GOOS {
	case "windows":
		configpath = filepath.Join(homeDir, ".config", "fansly-scraper", "config.json")
	default:
		configpath = filepath.Join(homeDir, ".config", "fansly-scraper", "config.json")
	}
	return configpath
}
