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
)

type mainModel struct {
	quit           bool
	cursorPos      int
	selected       string
	options        []string
	state          AppState
	followedModels []auth.FollowedModel
}

type userActionModel struct {
	welcome   string
	selected  string
	options   []string
	cursorPos int
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
			if m.selected == "Like all of a user's post" || m.selected == "Monitor a user's livestreams" || m.selected == "Download a user's post" || m.selected == "Unlike all of a user's post" {
				// Load the configuration
				cfg, err := config.LoadConfig(GetConfigPath())
				if err != nil {
					log.Printf("Error loading config: %v", err)
					return m, nil
				}

				// Switch view to followedModels list

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

			} else if m.selected == "Edit config.json file" {
				// Ensure the config.json file exists
				err := config.EnsureConfigExists(configPath)
				if err != nil {
					return m, nil
				}
				return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
					return tickMsg{}
				})
			} else if m.selected == "Quit" {
				m.quit = true
				return m, tea.Quit
			}
		}
		// return m, tea.Quit
		return m, nil
	default:
		return m, nil
	}
}

func (m *mainModel) View() string {
	var sb strings.Builder

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

	// Check if followed models are available and display them
	if len(m.followedModels) > 0 {
		sb.WriteString("\nFollowed Models:\n")
		for _, model := range m.followedModels {
			sb.WriteString(fmt.Sprintf("  %s | images: %d | videos: %d\n", model.Username, model.TimelineStats.ImageCount, model.TimelineStats.VideoCount))
		}
	}

	// sb.WriteString("\nSelected option: " + m.selected + "\n")

	return sb.String()
}

func main() {
	p := tea.NewProgram(&mainModel{
		options:   []string{"Download a user's post", "Monitor a user's livestreams", "Like all of a user's post", "Unlike all of a user's post", "Edit config.json file", "Quit"},
		cursorPos: 0,
	})
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
