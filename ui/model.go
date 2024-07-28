package ui

import (
	"github.com/agnosto/fansly-scraper/auth"
	//"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/download"

	//"fmt"
	//"log"
	//"os"
	//"path/filepath"
	//"runtime"
	//"strings"
	//"time"
	//"sync"
	"context"

	"github.com/schollz/progressbar/v3"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"

	//"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/lipgloss"
)

type AppState int

const (
    MainMenuState AppState = iota
    FollowedModelsState
    DownloadState
    DownloadActionsState 
    LiveMonitorState
    LikeUnlikeState 
    FilterState
    DownloadProgressState
)

type MainModel struct {
    version             string
	quit                bool
	cursorPos           int
	selected            string
	options             []string
	state               AppState
	followedModels      []auth.FollowedModel
    filteredModels      []auth.FollowedModel
    filterInput         string
    viewportStart       int
    viewportSize        int
    welcome             string
    table               table.Model
    keys                keyMap
    help                help.Model
    width               int
    height              int
    actionChosen        string
    downloadOptions     []string
    selectedOption      string
    selectedModel       string
    selectedModelId     string
    downloader          *download.Downloader
    progressBar         *progressbar.ProgressBar 
	cancelDownload      context.CancelFunc
    message             string
}

type fetchAccountInfoMsg struct {
    Success bool
    Error   error
    AccountInfo core.AccountInfo
}

type editConfigMsg struct {
    Success bool
    Error   error
}

type tickMsg struct{}

type downloadCompleteMsg struct{}

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

var defaultKeyMap = keyMap{
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

func (m *MainModel) Init() tea.Cmd {
	return nil
	// panic("unimplemented")
}

/*
func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	// Add cases for other states
	default:
		return m, nil
	}
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
	default:
		return "Unknown state"
	}
}
*/

func NewMainModel(downloader *download.Downloader, version string) *MainModel {
    return &MainModel{
        version: version,
        options:         []string{"Download a user's post", "Monitor a user's livestreams", "Like all of a user's post", "Unlike all of a user's post", "Edit config.json file", "Quit"},
        downloadOptions: []string{"All", "Timeline", "Messages", "Stories"},
        cursorPos:       0,
        keys:            defaultKeyMap,
        help:            help.New(),
        downloader:      downloader,
        state:           MainMenuState,
    }
}

func (m *MainModel) Reset() {
    m.cursorPos = 0
    m.selected = ""
    m.state = MainMenuState
    // Reset other fields as necessary
}
