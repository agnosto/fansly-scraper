package ui

import (
    "context"
    //"fmt"
    //"sync"
    "time"
    "strings"
    
    "github.com/agnosto/fansly-scraper/core"
    "github.com/agnosto/fansly-scraper/logger"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/table"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/bubbles/key"
)

func NewMonitoringService() *MonitoringService {
    return &MonitoringService{
        activeMonitors: make(map[string]context.CancelFunc),
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
                isLive, err := core.CheckIfModelIsLive(modelID)
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
    case tea.KeyMsg:
        switch {
        case key.Matches(msg, m.keys.Quit):
            m.quit = true
            return m, tea.Quit
        case key.Matches(msg, m.keys.Up):
            m.table.MoveUp(1)
        case key.Matches(msg, m.keys.Down):
            m.table.MoveDown(1)
        case key.Matches(msg, m.keys.Select):
            selectedRow := m.table.SelectedRow()
            modelID := selectedRow[1]
            username := selectedRow[0]
            if m.monitoredModels[modelID] {
                m.monitoringService.StopMonitoring(modelID)
                m.monitoredModels[modelID] = false
            } else {
                m.monitoringService.StartMonitoring(modelID, username)
                m.monitoredModels[modelID] = true
            }
            m.updateMonitoringTable()
        case key.Matches(msg, m.keys.Back):
            m.state = MainMenuState
            m.cursorPos = 0
        }
    }
    return m, nil
}

func (m *MainModel) RenderLivestreamMonitorMenu() string {
    var sb strings.Builder
    sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Render("Livestream Monitoring") + "\n\n")
    sb.WriteString(m.table.View() + "\n")
    helpView := m.help.View(m.keys)
    height := m.height - strings.Count(helpView, "\n") - m.table.Height() - 8 
    sb.WriteString("\n" + strings.Repeat("\n", height) + helpView)
    return sb.String()
}

func (m *MainModel) updateMonitoringTable() {
    columns := []table.Column{
        {Title: "Username", Width: 20},
        {Title: "Account ID", Width: 20},
        {Title: "Status", Width: 15},
    }

    rows := make([]table.Row, len(m.followedModels))
    for i, model := range m.followedModels {
        status := "Not Monitoring"
        if m.monitoredModels[model.ID] {
            status = "Monitoring"
        }
        rows[i] = table.Row{
            model.Username,
            model.ID,
            status,
        }
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithRows(rows),
        table.WithFocused(true),
        table.WithHeight(m.height - 10),
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
