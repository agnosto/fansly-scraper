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

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
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
    FilterState
)

type mainModel struct {
	quit            bool
	cursorPos       int
	selected        string
	options         []string
	state           AppState
	followedModels  []auth.FollowedModel
    filteredModels  []auth.FollowedModel
    filterInput     string
    viewportStart   int
    viewportSize    int
    welcome         string
    table           table.Model
    keys            keyMap
    help            help.Model
    width           int
    height          int
    actionChosen    string
}

type followedModelsModel struct {
	welcome         string
	selected        string
	followedModels  []auth.FollowedModel
	cursorPos       int
}

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Help    key.Binding
	Quit    key.Binding
    Filter  key.Binding
    Reset   key.Binding
    Back    key.Binding
    Select  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Filter}, // first column
		{k.Down, k.Back},      // second column
        {k.Help, k.Reset},
        {k.Quit, k.Select},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
    Filter: key.NewBinding(
        key.WithKeys("/"),
        key.WithHelp("/", "filter"),
    ),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
    Reset: key.NewBinding(
        key.WithKeys("r"),
        key.WithHelp("r", "reset list"),
    ),
    Back: key.NewBinding(
        key.WithKeys("esc"),
        key.WithHelp("esc", "back to menu"),
    ),
    Select: key.NewBinding(
        key.WithKeys("enter"),
        key.WithHelp("enter", "select"),
    ),
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
    case tea.WindowSizeMsg:
		m.help.Width = msg.Width
        m.width = msg.Width
		m.height = msg.Height 
        m.updateTable()
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
			switch {
			case key.Matches(msg, m.keys.Quit):
				m.quit = true
				return m, tea.Quit
			case key.Matches(msg, m.keys.Up):
				m.cursorPos = (m.cursorPos - 1 + len(m.options)) % len(m.options)
				return m, nil
			case key.Matches(msg, m.keys.Down):
				m.cursorPos = (m.cursorPos + 1) % len(m.options)
				return m, nil
			case key.Matches(msg, m.keys.Select):
				m.selected = m.options[m.cursorPos]
				switch m.selected {
				case "Download a user's post":
                    m.actionChosen = "download"
					m.state = DownloadState
                    m.fetchAccInfo(configPath)
					return m, nil
				case "Monitor a user's livestreams":
                    m.actionChosen = "monitor"
					m.state = LiveMonitorState
                    m.fetchAccInfo(configPath)
                    return m, nil 
				case "Like all of a user's post":
                    m.actionChosen = "like"
					m.state = LikePostState
                    m.fetchAccInfo(configPath)
                    return m, nil 
				case "Unlike all of a user's post":
                    m.actionChosen = "unlike"
					m.state = UnlikePostState
                    m.fetchAccInfo(configPath)
                    return m, nil 
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
				//selectedModel := m.followedModels[m.cursorPos]
				//fmt.Printf("Selected model: %s\n", selectedModel.Username)
                selectedRow := m.table.SelectedRow()
				fmt.Printf("Selected model: %s\n", selectedRow[0])
				// Handle post-download or other actions for the selected model here
				return m, nil
            case key.Matches(msg, m.keys.Filter):
                m.state = FilterState
                return m, nil
			case key.Matches(msg, m.keys.Back):
				m.state = MainMenuState
                m.cursorPos = 0
				return m, nil
			}
        case FilterState:
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
                m.table.MoveUp(1)
				return m, nil
			case "down":
                m.table.MoveDown(1)
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
	default:
		return m, nil
	}
	return m, nil
}

func (m *mainModel) View() string {
	var sb strings.Builder
    version := "0.0.5"

	switch m.state {
	case MainMenuState:
		// Welcome message
		configpath := GetConfigPath()
		styledConfigPath := lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render(configpath)
		welcomeMessage := "Config path: " + styledConfigPath + "\n" + "Welcome to Fansly-scraper Version " + version
		styledWelcomeMessage := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Render(welcomeMessage)
		sb.WriteString(styledWelcomeMessage + "\n")
		// Maintainer Repo
		repoLink := "https://github.com/agnosto/fansly-scraper"
		styledRepoLink := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Render(repoLink)
		sb.WriteString("Maintainer's repo: " + styledRepoLink + "\n\n")

		sb.WriteString("What would you like to do? " + m.selected + "\n")

		for i, opt := range m.options {
			if i == m.cursorPos {
				sb.WriteString("> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")).Render(opt) + "\n")
			} else {
				sb.WriteString("  " + opt + "\n")
			}
		}
        helpView := m.help.View(m.keys)
        sb.WriteString("\n" + helpView)
	    
	case FollowedModelsState:
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

    case FilterState:
        sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render(m.welcome) + "\n")
        sb.WriteString("Filter by username: " + m.filterInput + "\n")
        sb.WriteString(m.table.View() + "\n")

	}

	return sb.String()
}

func (m *mainModel) applyFilter() {
	filtered := []auth.FollowedModel{}
	for _, model := range m.followedModels {
		if strings.Contains(strings.ToLower(model.Username), strings.ToLower(m.filterInput)) {
			filtered = append(filtered, model)
		}
	}
	m.filteredModels = filtered
	m.updateTable()
}

func (m *mainModel) fetchAccInfo(configPath string) {
    // Load the configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
	    log.Printf("Error loading config: %v", err)
		return
	}

	// Fetch the account information using the auth token and user agent from the config
	accountInfo, err := auth.Login(cfg.Authorization, cfg.UserAgent)
	if err != nil {
		log.Println(cfg.Authorization)
		log.Printf("Error logging in: %v", err)
		return
	}
    m.welcome = fmt.Sprintf("Welcome %s | %s", accountInfo.DisplayName, accountInfo.Username)

	// Fetch the list of followed users
	followedModels, err := auth.GetFollowedUsers(accountInfo.ID, cfg.Authorization, cfg.UserAgent)
	if err != nil {
		fmt.Printf("\nError getting followed models: %v\n", err)
		return
	}
	m.followedModels = followedModels
    m.filteredModels = followedModels
    m.updateTable()
   	m.state = FollowedModelsState

}

func (m *mainModel) updateTable() {
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

func main() {
	p := tea.NewProgram(&mainModel{
		options:    []string{"Download a user's post", "Monitor a user's livestreams", "Like all of a user's post", "Unlike all of a user's post", "Edit config.json file", "Quit"},
		cursorPos:  0,
        keys:       keys,
        help:       help.New(),
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
