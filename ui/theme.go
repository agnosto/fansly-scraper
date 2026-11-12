package ui

import (
	"github.com/agnosto/fansly-scraper/config"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds all TUI styles, built from the [ui] section of the config.
// Empty config values fall back to the built-in defaults.
type Theme struct {
	Title    lipgloss.Style
	Accent   lipgloss.Style
	Selected lipgloss.Style
	Success  lipgloss.Style
	Error    lipgloss.Style
	Warning  lipgloss.Style
	Link     lipgloss.Style
	Muted    lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	Border   lipgloss.Color
	RowFg    lipgloss.Color
	RowBg    lipgloss.Color
	Live     lipgloss.Color
	Offline  lipgloss.Color
	// BarColor is the raw color used to fill the in-TUI progress bar.
	BarColor string
}

func colorOr(value, fallback string) lipgloss.Color {
	if value == "" {
		return lipgloss.Color(fallback)
	}
	return lipgloss.Color(value)
}

func fgStyle(value, fallback string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorOr(value, fallback))
}

// NewTheme builds a Theme from the [ui] config section, applying defaults
// for any unset color.
func NewTheme(c config.UIConfig) Theme {
	barColor := c.SelectedColor
	if barColor == "" {
		barColor = "#89dceb"
	}
	return Theme{
		Title:    fgStyle(c.TitleColor, "#f5c2e7"),
		Accent:   fgStyle(c.AccentColor, "#f5c2e7"),
		Selected: fgStyle(c.SelectedColor, "#89dceb"),
		BarColor: barColor,
		Success:  fgStyle(c.SuccessColor, "#a6e3a1"),
		Error:    fgStyle(c.ErrorColor, "#f38ba8"),
		Warning:  fgStyle(c.WarningColor, "#fab387"),
		Link:     fgStyle(c.LinkColor, "#89b4fa"),
		Muted:    fgStyle(c.MutedColor, "#6c7086"),
		HelpKey:  fgStyle(c.HelpKeyColor, "#89dceb"),
		HelpDesc: fgStyle(c.HelpDescColor, "#cdd6f4"),
		Border:   colorOr(c.BorderColor, "240"),
		RowFg:    colorOr(c.SelectedRowFgColor, "229"),
		RowBg:    colorOr(c.SelectedRowBgColor, "#cba6f7"),
		Live:     colorOr(c.LiveColor, "green"),
		Offline:  colorOr(c.OfflineColor, "red"),
	}
}

// DefaultTheme returns the theme used when no config is available.
func DefaultTheme() Theme {
	return NewTheme(config.UIConfig{})
}

// ApplyHelp applies the theme's tooltip colors to a help bubble, keeping the
// separator/ellipsis styling that help.New() set up.
func (t Theme) ApplyHelp(h *help.Model) {
	s := h.Styles
	s.ShortKey = t.HelpKey
	s.ShortDesc = t.HelpDesc
	s.FullKey = t.HelpKey
	s.FullDesc = t.HelpDesc
	h.Styles = s
}

// TableStyles returns table styles (header border + selected row) using the
// theme colors.
func (t Theme) TableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(t.RowFg).
		Background(t.RowBg).
		Bold(false)
	return s
}
