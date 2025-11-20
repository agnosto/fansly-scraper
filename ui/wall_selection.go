package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *MainModel) HandleWallSelectionUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			m.table.MoveUp(1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.table.MoveDown(1)
			return m, nil
		case key.Matches(msg, m.keys.Back):
			m.state = DownloadActionsState
			m.cursorPos = 0
			return m, nil
		case key.Matches(msg, m.keys.Select):
			selectedRow := m.table.SelectedRow()
			// The ID is hidden or in column 0 depending on how we set up the table
			// Let's assume ID is column 0
			if len(selectedRow) > 0 {
				m.selectedWallID = selectedRow[0]
				m.state = DownloadProgressState
				return m, m.startWallDownload()
			}
		}
	}
	return m, nil
}

func (m *MainModel) startWallDownload() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.downloader.DownloadTimeline(ctx, m.selectedModelId, m.selectedModel, m.selectedWallID)
		if err != nil {
			logger.Logger.Printf("Error downloading wall %s: %v", m.selectedWallID, err)
			return downloadErrorMsg{Error: err}
		}
		return downloadCompleteMsg{}
	}
}

func (m *MainModel) RenderWallSelectionMenu() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Select a wall to download for %s:\n", m.selectedModel))
	sb.WriteString(m.table.View() + "\n")
	helpView := m.help.View(m.keys)
	sb.WriteString("\n" + helpView)

	return sb.String()
}

// Call this when entering WallSelectionState
func (m *MainModel) updateWallTable() {
	columns := []table.Column{
		{Title: "ID", Width: 20},
		{Title: "Name", Width: 30},
		{Title: "Description", Width: 40},
	}

	// Find the currently selected model in the followedModels list to get their walls
	var currentModelWalls []auth.Wall
	for _, model := range m.followedModels {
		if model.ID == m.selectedModelId {
			currentModelWalls = make([]auth.Wall, len(model.Walls))
			copy(currentModelWalls, model.Walls)
			break
		}
	}

	// Sort by Position (Pos)
	sort.Slice(currentModelWalls, func(i, j int) bool {
		return currentModelWalls[i].Pos < currentModelWalls[j].Pos
	})

	rows := make([]table.Row, len(currentModelWalls))
	for i, wall := range currentModelWalls {
		rows[i] = table.Row{
			wall.ID,
			wall.Name,
			wall.Description,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
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
