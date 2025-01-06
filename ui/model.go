package ui

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/agnosto/fansly-scraper/auth"
	//"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/download"
	"github.com/agnosto/fansly-scraper/service"

	//"fmt"
	//"log"
	//"os"
	//"path/filepath"
	//"runtime"
	//"strings"
	//"time"
	"context"
	"sync"

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
	LiveMonitorFilterState
	DownloadPurchasedState
	CompletionState
)

type MainModel struct {
	version                   string
	quit                      bool
	cursorPos                 int
	selected                  string
	options                   []string
	state                     AppState
	followedModels            []auth.FollowedModel
	filteredModels            []auth.FollowedModel
	filterInput               string
	viewportStart             int
	viewportSize              int
	welcome                   string
	table                     table.Model
	monitoringTable           table.Model
	liveMonitorFilterInput    string
	filteredLiveMonitorModels []auth.FollowedModel
	keys                      keyMap
	help                      help.Model
	width                     int
	height                    int
	actionChosen              string
	downloadOptions           []string
	selectedOption            string
	selectedModel             string
	selectedModelId           string
	downloader                *download.Downloader
	progressBar               *progressbar.ProgressBar
	cancelDownload            context.CancelFunc
	message                   string
	monitoredModels           map[string]bool // Map of model IDs to monitoring status
	monitoringService         *service.MonitoringService
	program                   *tea.Program
}

type LiveStatusUpdateMsg struct{}

type delayedDownloadCompleteMsg struct{}
type delayedLikeUnlikeCompleteMsg struct{}

type monitoringSelectedMsg struct{}

type MonitoringService struct {
	activeMonitors map[string]context.CancelFunc
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
}

type fetchAccountInfoMsg struct {
	Success     bool
	Error       error
	AccountInfo core.AccountInfo
}

type editConfigMsg struct {
	Success bool
	Error   error
}

type editConfigFinishedMsg struct{}

type downloadErrorMsg struct {
	Error error
}

type tickMsg struct{}

type downloadCompleteMsg struct{}

type followedModelsModel struct {
	welcome        string
	selected       string
	followedModels []auth.FollowedModel
	cursorPos      int
}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Help   key.Binding
	Quit   key.Binding
	Filter key.Binding
	Reset  key.Binding
	Back   key.Binding
	Select key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Filter}, // first column
		{k.Down, k.Back}, // second column
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
	// Add signal handling
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg {
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-c
				m.Cleanup()
				os.Exit(0)
			}()
			return nil
		},
	)
}

func NewMainModel(downloader *download.Downloader, version string, monitoringService *service.MonitoringService) *MainModel {
	return &MainModel{
		version: version,
		options: []string{
			"Download a user's post",
			"Download purchased content",
			"Monitor a user's livestreams",
			"Like all of a user's post",
			"Unlike all of a user's post",
			"Edit config.toml file",
			"Quit",
		},
		downloadOptions:   []string{"All", "Timeline", "Messages", "Stories"},
		cursorPos:         0,
		keys:              defaultKeyMap,
		help:              help.New(),
		downloader:        downloader,
		state:             MainMenuState,
		monitoredModels:   make(map[string]bool),
		monitoringService: monitoringService,
	}
}

func (m *MainModel) Reset() {
	m.cursorPos = 0
	m.selected = ""
	m.state = MainMenuState
	// Reset other fields as necessary
}
