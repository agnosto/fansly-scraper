package ui

import (
	"testing"

	"github.com/agnosto/fansly-scraper/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewThemeDefaults(t *testing.T) {
	theme := NewTheme(config.UIConfig{})

	if got := theme.HelpDesc.GetForeground(); got != lipgloss.Color("#cdd6f4") {
		t.Errorf("expected default help desc color #cdd6f4, got %v", got)
	}
	if got := theme.HelpKey.GetForeground(); got != lipgloss.Color("#89dceb") {
		t.Errorf("expected default help key color #89dceb, got %v", got)
	}
	if got := theme.Border; got != lipgloss.Color("240") {
		t.Errorf("expected default border color 240, got %v", got)
	}
}

func TestNewThemeCustomColors(t *testing.T) {
	cfg := config.UIConfig{
		HelpDescColor: "#ffffff",
		HelpKeyColor:  "yellow",
		LiveColor:     "#ff0000",
	}
	theme := NewTheme(cfg)

	if got := theme.HelpDesc.GetForeground(); got != lipgloss.Color("#ffffff") {
		t.Errorf("expected custom help desc color #ffffff, got %v", got)
	}
	if got := theme.HelpKey.GetForeground(); got != lipgloss.Color("yellow") {
		t.Errorf("expected custom help key color yellow, got %v", got)
	}
	if got := theme.Live; got != lipgloss.Color("#ff0000") {
		t.Errorf("expected custom live color #ff0000, got %v", got)
	}
	// Unset fields still fall back to defaults
	if got := theme.Selected.GetForeground(); got != lipgloss.Color("#89dceb") {
		t.Errorf("expected default selected color #89dceb, got %v", got)
	}
}

func TestDefaultThemeMatchesEmptyConfig(t *testing.T) {
	def := DefaultTheme()
	empty := NewTheme(config.UIConfig{})
	if def.HelpDesc.GetForeground() != empty.HelpDesc.GetForeground() ||
		def.Selected.GetForeground() != empty.Selected.GetForeground() ||
		def.Border != empty.Border {
		t.Errorf("DefaultTheme should match NewTheme with an empty config")
	}
}

func TestEmbeddedWizardDoesNotQuitProgram(t *testing.T) {
	w := NewConfigWizardModel()
	w.Embedded = true

	model, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	w = model.(*ConfigWizardModel)
	if !w.done {
		t.Errorf("expected embedded wizard to set done on esc")
	}
	if cmd != nil {
		t.Errorf("expected embedded wizard to return nil cmd on esc (must not tea.Quit), got %v", cmd)
	}
}

func TestStandaloneWizardQuitsProgram(t *testing.T) {
	w := NewConfigWizardModel()

	model, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	w = model.(*ConfigWizardModel)
	if w.done {
		t.Errorf("standalone wizard must not set done")
	}
	if cmd == nil {
		t.Errorf("expected standalone wizard to return tea.Quit cmd on esc")
	}
}
