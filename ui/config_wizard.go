package ui

import (
	"path/filepath"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type ConfigWizardModel struct {
	state   int
	inputs  [3]textinput.Model
	cursor  int
	message string
}

const (
	wizardSaveLocation = iota
	wizardUserAgent
	wizardAuthToken
)

func NewConfigWizardModel() *ConfigWizardModel {
	m := &ConfigWizardModel{}
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "Save location (folder)"
	m.inputs[0].Focus()
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "HTTP User-Agent"
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "Auth token"
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[2].EchoCharacter = '•'
	return m
}

func (m *ConfigWizardModel) Init() tea.Cmd { return textinput.Blink }

func (m *ConfigWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.message = "Setup cancelled."
			return m, tea.Quit
		case "enter":
			if m.cursor < len(m.inputs)-1 {
				m.cursor++
				m.inputs[m.cursor].Focus()
				return m, nil
			}
			cfg := config.CreateDefaultConfig()
			cfg.Options.SaveLocation = filepath.Clean(m.inputs[0].Value())
			cfg.Account.UserAgent = m.inputs[1].Value()
			cfg.Account.AuthToken = m.inputs[2].Value()
			if err := config.ValidateConfig(cfg, config.GetConfigPath()); err != nil {
				m.message = err.Error()
				return m, nil
			}
			if err := config.EnsureConfigExists(config.GetConfigPath()); err != nil {
				m.message = err.Error()
				return m, nil
			}
			if err := config.SaveConfig(cfg); err != nil {
				m.message = err.Error()
				return m, nil
			}
			return m, tea.Quit
		case "tab":
			m.cursor = (m.cursor + 1) % len(m.inputs)
			for i := range m.inputs {
				if i == m.cursor {
					m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
		}
	}

	for i := range m.inputs {
		m.inputs[i], _ = m.inputs[i].Update(msg)
	}
	return m, nil
}

func (m *ConfigWizardModel) View() string {
	v := "First-time minimal setup: create config.toml\n\n"
	v += "Windows: use forward slashes (C:/path/to/dir) or right-click to paste then escape backslashes (C:\\\\path\\\\to\\\\dir).\n\n"
	v += m.inputs[0].View() + "\n"
	v += m.inputs[1].View() + "\n"
	v += m.inputs[2].View() + "\n\n"
	if m.message != "" {
		v += m.message + "\n"
	}
	v += "Press Enter to save, Tab to switch, Esc to quit. More settings under 'Edit config.toml file' in the main menu.\n"
	return v
}
