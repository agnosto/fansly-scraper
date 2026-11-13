package ui

import (
	"strings"
	//"fmt"
	"github.com/agnosto/fansly-scraper/auth"
	//"github.com/agnosto/fansly-scraper/config"
	//"github.com/agnosto/fansly-scraper/core"

	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/bubbles/key"
	//"github.com/charmbracelet/bubbles/table"
)

// fullRepaintCmd re-posts the current window size, which resets the
// renderer's frame cache so the next flush redraws every line in place
// without erasing the screen (avoids the flicker of tea.ClearScreen).
// Used because rebuilding the filter table can leave stale styled rows
// behind on some Windows terminals.
func fullRepaintCmd(width, height int) tea.Cmd {
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: width, Height: height}
	}
}

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleFilterModelsMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "esc":
			m.filteredModels = m.followedModels // Reset to unfiltered list
			m.filterInput = ""                  // Reset filter input
			m.updateTable()                     // Update table to show unfiltered list
			m.state = FollowedModelsState
			return m, nil
		case "up":
			//    m.table.MoveUp(1)
			return m, nil
		case "down":
			//    m.table.MoveDown(1)
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
			// Rebuilding the table shrinks/expands rows in place; on some
			// Windows terminals the line-diff renderer leaves the previously
			// styled row painted, so force a full (flicker-free) repaint.
			return m, fullRepaintCmd(m.width, m.height)
		default:
			// Only accept printable single-character keys; multi-key strings
			// like "ctrl+a" or literal "space" must not leak into the input.
			if s := msg.String(); len([]rune(s)) == 1 {
				m.filterInput += s
				m.applyFilter()
			}
			return m, fullRepaintCmd(m.width, m.height)
		}
	}
	return m, nil
}

func (m *MainModel) RenderFilterModelsMenu() string {
	var sb strings.Builder

	sb.WriteString(m.theme.Title.Render(m.welcome) + "\n")
	sb.WriteString("Filter by username: " + m.filterInput + "\n")
	sb.WriteString(m.table.View() + "\n")

	return sb.String()
}

func (m *MainModel) applyFilter() {
	filtered := []auth.FollowedModel{}
	for _, model := range m.followedModels {
		if strings.Contains(strings.ToLower(model.Username), strings.ToLower(m.filterInput)) {
			filtered = append(filtered, model)
		}
	}
	m.filteredModels = filtered
	m.updateTable()
}
