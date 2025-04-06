package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// RenderLoadingScreen renders a loading animation
func (m *MainModel) RenderLoadingScreen() string {
	var sb strings.Builder

	// Create a centered loading message
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f5c2e7")).
		Bold(true).
		Padding(2, 0, 1, 0)

	dots := strings.Repeat(".", m.loadingDots)
	loadingText := fmt.Sprintf("%s%s", m.loadingMessage, dots)

	// Center the loading message
	centeredLoading := lipgloss.Place(
		m.width,
		m.height/2,
		lipgloss.Center,
		lipgloss.Center,
		loadingStyle.Render(loadingText),
	)

	sb.WriteString(centeredLoading)

	return sb.String()
}
