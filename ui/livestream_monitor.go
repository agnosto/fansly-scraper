package ui

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	//"fmt"
	//"sync"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/logger"

	//"github.com/agnosto/fansly-scraper/service"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func NewMonitoringService() *MonitoringService {
	ctx, cancel := context.WithCancel(context.Background())
	return &MonitoringService{
		activeMonitors: make(map[string]context.CancelFunc),
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (ms *MonitoringService) Shutdown() {
	ms.cancel()
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, cancel := range ms.activeMonitors {
		cancel()
	}
}

func (ms *MonitoringService) StartMonitoring(modelID, username string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.activeMonitors[modelID]; exists {
		return // Already monitoring
	}

	ctx, cancel := context.WithCancel(context.Background())
	ms.activeMonitors[modelID] = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Check if the model is live
				isLive, _, err := core.CheckIfModelIsLive(modelID)
				if err != nil {
					logger.Logger.Printf("Error checking if %s is live: %v", username, err)
				} else if isLive {
					logger.Logger.Printf("%s is now live!", username)
					// Here you would implement the logic to start recording
					// This could involve calling a function from your downloader package
				}
				time.Sleep(2 * time.Minute) // Check every 2 minutes
			}
		}
	}()
}

func (ms *MonitoringService) StopMonitoring(modelID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if cancel, exists := ms.activeMonitors[modelID]; exists {
		cancel()
		delete(ms.activeMonitors, modelID)
	}
}

func (m *MainModel) HandleLivestreamMonitorUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LiveStatusUpdateMsg:
		m.updateMonitoringTable()
		return m, tea.Batch(
			tea.Tick(time.Minute*2, func(t time.Time) tea.Msg {
				return LiveStatusUpdateMsg{}
			}),
		)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit), msg.String() == "ctrl+c":
			m.quit = true
			m.Cleanup()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Reset):
			m.filteredLiveMonitorModels = m.followedModels
			m.liveMonitorFilterInput = ""
			m.updateMonitoringTable()
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.monitoringTable.MoveUp(1)
		case key.Matches(msg, m.keys.Down):
			m.monitoringTable.MoveDown(1)
		case key.Matches(msg, m.keys.Select):
			selectedRow := m.monitoringTable.SelectedRow()
			modelID := selectedRow[1]
			username := selectedRow[0]
			m.monitoringService.ToggleMonitoring(modelID, username)
			m.updateMonitoringTable()
		case key.Matches(msg, m.keys.Filter):
			m.state = LiveMonitorFilterState
			return m, nil
		case key.Matches(msg, m.keys.Back):
			m.state = MainMenuState
			m.cursorPos = 0
			return m, nil
			//case msg.String() == "backspace":
			//    if len(m.liveMonitorFilterInput) > 0 {
			//        m.liveMonitorFilterInput = m.liveMonitorFilterInput[:len(m.liveMonitorFilterInput)-1]
			//        m.applyLiveMonitorFilter()
			//    }
			//default:
			//    m.liveMonitorFilterInput += msg.String()
			//    m.applyLiveMonitorFilter()
		}
	}
	return m, nil
}

func (m *MainModel) HandleLiveMonitorFilterUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "esc":
			m.filteredLiveMonitorModels = m.followedModels // Reset to unfiltered list
			m.liveMonitorFilterInput = ""                  // Reset filter input
			m.updateMonitoringTable()                      // Update table to show unfiltered list
			m.state = LiveMonitorState
			return m, nil
		case "up":
			//    m.table.MoveUp(1)
			return m, nil
		case "down":
			//    m.table.MoveDown(1)
			return m, nil
		case "enter":
			m.applyLiveMonitorFilter()
			m.state = LiveMonitorState
			m.liveMonitorFilterInput = ""
			return m, nil
		case "backspace":
			if len(m.liveMonitorFilterInput) > 0 {
				m.liveMonitorFilterInput = m.liveMonitorFilterInput[:len(m.liveMonitorFilterInput)-1]
				m.applyLiveMonitorFilter()
			}
		default:
			m.liveMonitorFilterInput += msg.String()
			m.applyLiveMonitorFilter()
			return m, nil
		}
	}
	return m, nil
}

func (m *MainModel) RenderLivestreamMonitorMenu() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render("Livestream Monitoring") + "\n\n")
	sb.WriteString(m.monitoringTable.View() + "\n")
	helpView := m.help.View(m.keys)
	height := m.height - strings.Count(helpView, "\n") - m.monitoringTable.Height() - 8
	sb.WriteString("\n" + strings.Repeat("\n", height) + helpView)
	return sb.String()
}

func (m *MainModel) RenderLiveMonitorFilterMenu() string {
	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render("Livestream Monitoring Filter") + "\n\n")
	sb.WriteString("Filter by username: " + m.liveMonitorFilterInput + "\n")
	sb.WriteString(m.monitoringTable.View() + "\n")

	//helpView := m.help.View(m.keys)
	//height := m.height - strings.Count(helpView, "\n") - m.monitoringTable.Height() - 8
	//sb.WriteString("\n" + strings.Repeat("\n", height) + helpView)

	return sb.String()
}

func (m *MainModel) initializeLivestreamMonitoringTable() tea.Cmd {
	m.filteredLiveMonitorModels = m.followedModels
	m.monitoringService.SetTUIMode(true)
	m.monitoringService.StartMonitoring()
	m.updateMonitoringTable()
	return tea.Batch(
		m.startLiveStatusUpdates(),
		func() tea.Msg { return LiveStatusUpdateMsg{} },
	)
}

func (m *MainModel) loadMonitoringState() map[string]string {
	configDir := config.GetConfigDir()
	statePath := filepath.Join(configDir, "monitoring_state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Logger.Printf("Error reading monitoring state: %v", err)
		}
		return make(map[string]string)
	}

	var activeMonitors map[string]string
	if err := json.Unmarshal(data, &activeMonitors); err != nil {
		logger.Logger.Printf("Error parsing monitoring state: %v", err)
		return make(map[string]string)
	}

	return activeMonitors
}

func (m *MainModel) updateMonitoringTable() {
	columns := []table.Column{
		{Title: "Username", Width: 20},
		{Title: "Account ID", Width: 20},
		{Title: "Monitor Status", Width: 15},
		{Title: "Live Status", Width: 15},
	}

	activeMonitors := m.loadMonitoringState()
	rows := make([]table.Row, len(m.filteredLiveMonitorModels))

	for i, model := range m.filteredLiveMonitorModels {
		monitorStatus := "Not Monitoring"
		monitorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("red"))
		liveStatus := "Offline"
		liveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("red"))

		if _, isMonitored := activeMonitors[model.ID]; isMonitored {
			monitorStatus = "Monitoring"
			monitorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))

			isLive, _, _ := core.CheckIfModelIsLive(model.ID)
			if isLive {
				liveStatus = "Live"
				liveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
			}
		}

		rows[i] = table.Row{
			model.Username,
			model.ID,
			monitorStyle.Render(monitorStatus),
			liveStyle.Render(liveStatus),
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(m.height-10),
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

	m.monitoringTable = t
}

func (m *MainModel) applyLiveMonitorFilter() {
	filtered := []auth.FollowedModel{}
	for _, model := range m.followedModels {
		if strings.Contains(strings.ToLower(model.Username), strings.ToLower(m.liveMonitorFilterInput)) {
			filtered = append(filtered, model)
		}
	}
	m.filteredLiveMonitorModels = filtered
	m.updateMonitoringTable()
}

func (m *MainModel) startLiveStatusUpdates() tea.Cmd {
	return tea.Tick(time.Minute*2, func(t time.Time) tea.Msg {
		return LiveStatusUpdateMsg{}
	})
}

func (m *MainModel) Cleanup() {
	// Stop all monitoring first
	m.monitoringService.Shutdown()

	// Kill FFmpeg processes with proper error handling
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("taskkill", "/F", "/IM", "ffmpeg.exe")
	} else {
		cmd = exec.Command("pkill", "ffmpeg")
	}
	cmd.Run() // Execute the command synchronously

	// Clean up lock files with verification
	recordingsPath := filepath.Join(config.GetConfigDir(), "active_recordings")
	files, err := os.ReadDir(recordingsPath)
	if err == nil {
		for _, file := range files {
			if filepath.Ext(file.Name()) == ".lock" {
				lockFile := filepath.Join(recordingsPath, file.Name())
				if err := os.Remove(lockFile); err != nil {
					logger.Logger.Printf("Failed to remove lock file %s: %v", lockFile, err)
				}
			}
		}
	}
}

// Add a cleanup function that can be called from the TUI
/*
func cleanupLockFiles() {
	recordingsPath := filepath.Join(config.GetConfigDir(), "active_recordings")
	files, err := os.ReadDir(recordingsPath)
	if err != nil {
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".lock" {
			lockFile := filepath.Join(recordingsPath, file.Name())
			os.Remove(lockFile)
		}
	}
}
*/
