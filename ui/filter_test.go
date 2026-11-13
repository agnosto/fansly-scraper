package ui

import (
	"testing"

	"github.com/agnosto/fansly-scraper/auth"
	tea "github.com/charmbracelet/bubbletea"
)

func fakeFollowedModels(n int) []auth.FollowedModel {
	names := []string{"alphauser", "bravo", "gammauser", "delta", "albertuser", "brian"}
	models := make([]auth.FollowedModel, 0, n)
	for i := 0; i < n && i < len(names); i++ {
		models = append(models, auth.FollowedModel{
			ID:       names[i],
			Username: names[i],
		})
	}
	return models
}

// TestFilterKeepsMarkerInSync ensures that typing in the filter screen keeps
// the leading > indicator on the same row as the table's cursor, so only one
// row ever appears selected.
func TestFilterKeepsMarkerInSync(t *testing.T) {
	m := NewMainModel(nil, "test", nil, nil)
	m.width, m.height = 115, 60
	m.followedModels = fakeFollowedModels(6)
	m.filteredModels = m.followedModels
	m.actionChosen = "download"
	m.updateTable()
	m.state = FilterState

	// Cursor sits on row 3 when the filter is opened.
	for i := 0; i < 3; i++ {
		m.HandleFollowedModelsMenuUpdate(tea.KeyMsg{Type: tea.KeyDown})
	}

	type charMsg struct {
		key tea.KeyMsg
	}

	for _, r := range []charMsg{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}},
		{tea.KeyMsg{Type: tea.KeyBackspace}},
		{tea.KeyMsg{Type: tea.KeySpace}},
	} {
		m.HandleFilterModelsMenuUpdate(r.key)
		cursor := m.table.Cursor()
		markers := 0
		markerIdx := -1
		for i, row := range m.table.Rows() {
			if row[0] == ">" {
				markers++
				markerIdx = i
			}
		}
		if len(m.table.Rows()) > 0 && markers != 1 {
			t.Fatalf("after key %q: %d marker rows", r.key.String(), markers)
		}
		if markers == 1 && markerIdx != cursor {
			t.Fatalf("after key %q: cursor=%d but marker on row %d", r.key.String(), cursor, markerIdx)
		}
	}

	// Non-printable multi-key strings must not leak into the filter input.
	m.HandleFilterModelsMenuUpdate(tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.filterInput != "a " {
		t.Fatalf("filterInput = %q", m.filterInput)
	}
}
