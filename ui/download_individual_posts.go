package ui

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/agnosto/fansly-scraper/logger"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HandleDownloadIndividualPostsUpdate handles updates when in the DownloadIndividualPostState
func (m *MainModel) HandleDownloadIndividualPostsUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.state = MainMenuState
			m.cursorPos = 0
			return m, nil
		case key.Matches(msg, m.keys.Select):
			// Enter pressed: start download process if not empty
			val := strings.TrimSpace(m.postLinksInput.Value())
			if val != "" {
				m.state = DownloadProgressState
				return m, m.startIndividualPostsDownload(val)
			}
			return m, nil
		}
	}

	m.postLinksInput, cmd = m.postLinksInput.Update(msg)
	return m, cmd
}

// RenderDownloadIndividualPostsMenu renders the input menu for individual post links
func (m *MainModel) RenderDownloadIndividualPostsMenu() string {
	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Bold(true).Render("Download Individual Post(s)") + "\n\n")
	sb.WriteString("Paste one or more Fansly post links or IDs below.\n")
	sb.WriteString("You can separate multiple links/IDs with spaces or commas.\n\n")

	sb.WriteString(m.postLinksInput.View() + "\n\n")

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Render("Press Enter to download, Esc to cancel / return to menu") + "\n")

	return sb.String()
}

// parseIndividualPostLinks splits input string by space, comma, semicolon, or newlines and extracts post IDs/URLs.
func parseIndividualPostLinks(input string) []string {
	var postIDs []string
	rawTokens := strings.FieldsFunc(input, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	for _, token := range rawTokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		// Extract post ID if it's a URL
		postID := token
		if strings.Contains(token, "/") {
			parts := strings.Split(token, "/")
			postID = strings.Split(parts[len(parts)-1], "?")[0]
		}
		// Basic validation: must be numeric
		if _, err := strconv.ParseUint(postID, 10, 64); err == nil {
			postIDs = append(postIDs, postID)
		}
	}
	return postIDs
}

// startIndividualPostsDownload starts the download of multiple post IDs/URLs asynchronously
func (m *MainModel) startIndividualPostsDownload(input string) tea.Cmd {
	return func() tea.Msg {
		postIDs := parseIndividualPostLinks(input)
		if len(postIDs) == 0 {
			logger.Logger.Printf("No valid post IDs or URLs found in input: %q", input)
			return downloadCompleteMsg{}
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			ctx := context.Background()

			for i, id := range postIDs {
				logger.Logger.Printf("Processing individual post %d/%d (ID: %s)", i+1, len(postIDs), id)
				if err := m.downloader.DownloadPostByID(ctx, id); err != nil {
					logger.Logger.Printf("Failed to download individual post %s: %v", id, err)
				}
			}
		}()

		wg.Wait()
		return downloadCompleteMsg{}
	}
}
