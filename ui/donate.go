package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mdp/qrterminal/v3"
)

const (
	btcAddress         = "bc1q0e78wrtc9ezp6tqv000wfewgqf2ue4tpzdk7ee"
	solAddress         = "Bv3kYZcwSTHXAQtnPddTF27D3F6Gc29v2MfFLqmGF6Gf"
	githubSponsorsLink = "https://github.com/sponsors/agnosto" // Updated Link
)

// HandleDonateMenuUpdate handles input for the donation screen
func (m *MainModel) HandleDonateMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.state = MainMenuState
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// createDonateColumn is a helper function to generate the content for a single crypto column.
func createDonateColumn(header, address string, headerStyle, addressStyle lipgloss.Style) string {
	var columnBuilder strings.Builder

	// Write the header for the column
	columnBuilder.WriteString(headerStyle.Render(header) + "\n")

	// Generate the QR code into a temporary builder, then add it to our column
	var qrBuilder strings.Builder
	qrConfig := qrterminal.Config{
		Level:      qrterminal.L,
		Writer:     &qrBuilder,
		HalfBlocks: true, // The key change is here! This makes the QR code compact.
		QuietZone:  1,
	}
	qrterminal.GenerateWithConfig(address, qrConfig)
	columnBuilder.WriteString(qrBuilder.String())

	// Write the address string at the bottom
	columnBuilder.WriteString("\n" + addressStyle.Render(address))

	return columnBuilder.String()
}

// RenderDonateMenu renders the donation screen with side-by-side QR codes
func (m *MainModel) RenderDonateMenu() string {
	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a6e3a1")).PaddingBottom(1)
	addressHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89dceb"))
	addressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe")).PaddingTop(1)
	sponsorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa"))
	columnSeparator := "    " // 4 spaces between the columns

	// Create each column's content as a self-contained string block
	btcColumn := createDonateColumn("Bitcoin (BTC)", btcAddress, addressHeaderStyle, addressStyle)
	solColumn := createDonateColumn("Solana (SOL)", solAddress, addressHeaderStyle, addressStyle)

	// With compact QR codes, we can now reliably use a horizontal layout.
	joinedColumns := lipgloss.JoinHorizontal(lipgloss.Top, btcColumn, columnSeparator, solColumn)

	// Updated message for GitHub Sponsors
	sponsorsMessage := fmt.Sprintf("\n\nYou can also support me on GitHub Sponsors: %s", sponsorStyle.Render(githubSponsorsLink))

	// Build the final view
	var finalView strings.Builder
	finalView.WriteString(titleStyle.Render("Support the Project") + "\n")
	finalView.WriteString("If you find this tool useful, please consider a donation. It helps a lot!\n\n")
	finalView.WriteString(joinedColumns)
	finalView.WriteString(sponsorsMessage) // Add the GitHub Sponsors message
	finalView.WriteString("\n" + helpStyle.Render("\nPress 'esc' to return to the main menu."))

	// Center the whole block on the screen
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		finalView.String(),
	)
}
