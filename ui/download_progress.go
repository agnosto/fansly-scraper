package ui

import (
	"fmt"
	"strings"
	"time"
	//"github.com/agnosto/fansly-scraper/config"
	//"github.com/agnosto/fansly-scraper/core"

	"github.com/agnosto/fansly-scraper/download"
	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/key"
	//"github.com/charmbracelet/bubbles/table"
	"github.com/dustin/go-humanize"
)

// HandleMainMenuUpdate handles updates when in the MainMenuState
func (m *MainModel) HandleDownloadProgressMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.state = FollowedModelsState
			m.cursorPos = 0
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		}
	}
	if !m.progressTicking {
		m.progressTicking = true
		return m, progressTickCmd()
	}
	return m, nil
}

// RenderMainMenu renders the main menu view
func (m *MainModel) RenderDownloadProgressMenu() string {
	var sb strings.Builder

	if m.actionChosen == "download_individual_posts" {
		sb.WriteString("Downloading individual post(s)...\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("Downloading %s content for %s...\n\n", m.selectedOption, m.selectedModel))
	}
	sb.WriteString(m.renderProgressStatus())

	return sb.String()
}

// renderProgressStatus renders the downloader's live progress inline as a
// colored progress bar plus a severity-colored status line.
func (m *MainModel) renderProgressStatus() string {
	if m.downloader == nil {
		return ""
	}
	st := m.downloader.ProgressStatus()
	if st.Description == "" && st.Total == 0 {
		return ""
	}

	descStyle := m.theme.Muted
	switch st.Level {
	case "error":
		descStyle = m.theme.Error
	case "warn":
		descStyle = m.theme.Warning
	case "info":
		descStyle = m.theme.Success
	}

	var sb strings.Builder
	if st.Total > 0 {
		pct := float64(st.Current) / float64(st.Total)
		if pct > 1 {
			pct = 1
		}
		if pct < 0 {
			pct = 0
		}
		sb.WriteString(m.progressView.ViewAs(pct) + "\n")
		sb.WriteString(descStyle.Render(fmt.Sprintf("%s (%d/%d)%s", st.Description, st.Current, st.Total, byteProgress(st))) + "\n")
	} else {
		// Total unknown (fetch phase): animate the bar so the user sees
		// the operation is running, with a live fetch count and elapsed time.
		sb.WriteString(m.progressView.ViewAs(m.fetchAnim) + "\n")
		line := st.Description
		if st.Current > 0 {
			line = fmt.Sprintf("%s (%d)", line, st.Current)
		}
		line += byteProgress(st)
		if !st.StartedAt.IsZero() {
			line += fmt.Sprintf(" [%s]", time.Since(st.StartedAt).Round(time.Second))
		}
		sb.WriteString(descStyle.Render(line) + "\n")
	}
	return sb.String()
}

// byteProgress formats the in-flight file's byte progress, e.g. " (1.2 MB/4.5 MB)".
func byteProgress(st download.ProgressStatus) string {
	switch {
	case st.BytesTotal > 0:
		return fmt.Sprintf(" (%s/%s)", humanize.Bytes(uint64(st.Bytes)), humanize.Bytes(uint64(st.BytesTotal)))
	case st.Bytes > 0:
		return fmt.Sprintf(" (%s)", humanize.Bytes(uint64(st.Bytes)))
	}
	return ""
}
