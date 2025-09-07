package ui

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "github.com/agnosto/fansly-scraper/auth"
    "github.com/agnosto/fansly-scraper/config"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/atotto/clipboard"
    "os/exec"
)

type ConfigWizardModel struct {
    state   int
    inputs  [3]textinput.Model
    cursor  int
    message string
    // Location step
    locOptions   []string
    locCursor    int
    locCustom    bool
    locInput     textinput.Model
    locMessage   string
    selectedPath string
    // Auto-capture state
    autoActive bool
    autoPort   int
    snippet    string
    waiting    bool
    stopFn     func(ctx context.Context) error
}

const (
    wizardStepLocation = iota
    wizardStepLogin
)

func NewConfigWizardModel() *ConfigWizardModel {
    m := &ConfigWizardModel{}
    m.state = wizardStepLocation
    // Pre-create inputs for login step
    m.inputs[0] = textinput.New()
    m.inputs[0].Placeholder = "Save location (folder)"
    m.inputs[1] = textinput.New()
    m.inputs[1].Placeholder = "HTTP User-Agent"
    m.inputs[2] = textinput.New()
    m.inputs[2].Placeholder = "Auth token"
    m.inputs[2].EchoMode = textinput.EchoPassword
    m.inputs[2].EchoCharacter = '•'

    // Location selection setup
    m.locInput = textinput.New()
    m.locInput.Placeholder = "Custom save location"

    // Build options based on OS
    exePath, _ := os.Executable()
    exeDir := filepath.Dir(exePath)
    home, _ := os.UserHomeDir()
    downloads := filepath.Join(home, "Downloads", "fansly")
    exeDefault := filepath.Join(exeDir, "content")
    homeDefault := filepath.Join(home, "fansly-content")
    m.locOptions = []string{
        fmt.Sprintf("Use exe folder (create): %s", exeDefault),
        fmt.Sprintf("Use home folder: %s", homeDefault),
        fmt.Sprintf("Use Downloads: %s", downloads),
        "Enter custom path...",
    }
    m.locCursor = 0
    return m
}

func (m *ConfigWizardModel) Init() tea.Cmd { return textinput.Blink }

func (m *ConfigWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch m.state {
        case wizardStepLocation:
            switch msg.String() {
            case "ctrl+c":
                if m.stopFn != nil {
                    _ = m.stopFn(context.Background())
                    m.stopFn = nil
                }
                return m, tea.Quit
            case "esc":
                if m.stopFn != nil {
                    _ = m.stopFn(context.Background())
                    m.stopFn = nil
                }
                m.message = "Setup cancelled."
                return m, tea.Quit
            case "up":
                if !m.locCustom {
                    m.locCursor = (m.locCursor - 1 + len(m.locOptions)) % len(m.locOptions)
                }
            case "down":
                if !m.locCustom {
                    m.locCursor = (m.locCursor + 1) % len(m.locOptions)
                }
            case "enter":
                if m.locCustom {
                    path := strings.TrimSpace(m.locInput.Value())
                    if path == "" {
                        m.locMessage = "Please enter a valid path."
                        return m, nil
                    }
                    m.selectedPath = filepath.Clean(path)
                    if err := os.MkdirAll(m.selectedPath, os.ModePerm); err != nil {
                        m.locMessage = fmt.Sprintf("Failed to create folder: %v", err)
                        return m, nil
                    }
                    // proceed to login step
                    m.inputs[0].SetValue(m.selectedPath)
                    m.state = wizardStepLogin
                    m.inputs[1].Focus()
                    m.locMessage = ""
                    return m, nil
                }
                // Not in custom mode: process selection
                switch m.locCursor {
                case 0, 1, 2:
                    // Parse the path portion from the option label after ': '
                    parts := strings.SplitN(m.locOptions[m.locCursor], ": ", 2)
                    if len(parts) == 2 {
                        m.selectedPath = filepath.Clean(parts[1])
                    }
                    if m.selectedPath == "" {
                        m.locMessage = "Internal error: empty path"
                        return m, nil
                    }
                    if err := os.MkdirAll(m.selectedPath, os.ModePerm); err != nil {
                        m.locMessage = fmt.Sprintf("Failed to create folder: %v", err)
                        return m, nil
                    }
                    m.inputs[0].SetValue(m.selectedPath)
                    m.state = wizardStepLogin
                    m.inputs[1].Focus()
                    return m, nil
                case 3:
                    // Enable custom input
                    m.locCustom = true
                    m.locInput.Focus()
                    return m, nil
                }
            }
        case wizardStepLogin:
            switch msg.String() {
            case "ctrl+c":
                return m, tea.Quit
            case "esc":
                m.message = "Setup cancelled."
                return m, tea.Quit
            case "a":
                if !m.autoActive {
                    m.message = "Starting browser auto-capture..."
                    m.waiting = true
                    return m, m.startAutoCaptureCmd()
                }
            case "o":
                _ = openURL("https://fansly.com/")
                return m, nil
            case "c":
                if m.snippet != "" {
                    if err := clipboard.WriteAll(m.snippet); err != nil {
                        m.message = "Failed to copy snippet to clipboard."
                    } else {
                        m.message = "Snippet copied. Paste in Fansly DevTools Console and press Enter."
                    }
                    return m, nil
                }
            case "enter":
                if m.cursor == 0 { // ensure cursor starts on UA field
                    m.cursor = 1
                }
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
    case autoCaptureStartedMsg:
        m.autoActive = true
        m.autoPort = msg.Port
        m.stopFn = msg.Stop
        m.snippet = buildCaptureSnippet(msg.Port)
        // After starting, wait for result
        return m, m.waitForCaptureCmd(msg.Ch)
    case autoCaptureResultMsg:
        m.waiting = false
        if msg.Err != nil {
            m.message = fmt.Sprintf("Auto-capture failed: %v", msg.Err)
            return m, nil
        }
        // Fill inputs automatically
        m.inputs[1].SetValue(msg.UserAgent)
        m.inputs[2].SetValue(msg.Token)
        m.message = "Captured token and user-agent from browser. Press Enter to save."
        // Stop server
        if m.stopFn != nil {
            _ = m.stopFn(context.Background())
            m.stopFn = nil
        }
        return m, nil
    }

    switch m.state {
    case wizardStepLocation:
        if m.locCustom {
            m.locInput, _ = m.locInput.Update(msg)
        }
    case wizardStepLogin:
        for i := range m.inputs {
            m.inputs[i], _ = m.inputs[i].Update(msg)
        }
    }
    return m, nil
}

func (m *ConfigWizardModel) View() string {
    var b strings.Builder
    switch m.state {
    case wizardStepLocation:
        b.WriteString("First-time setup: choose save location\n\n")
        for i, opt := range m.locOptions {
            if i == m.locCursor && !m.locCustom {
                b.WriteString("> " + opt + "\n")
            } else {
                b.WriteString("  " + opt + "\n")
            }
        }
        b.WriteString("\n")
        if m.locCustom {
            b.WriteString("Custom path: " + m.locInput.View() + "\n")
            b.WriteString("Press Enter to confirm, Esc to cancel.\n")
        } else {
            b.WriteString("Use Up/Down to select, Enter to confirm. Esc to quit.\n")
        }
        if m.locMessage != "" {
            b.WriteString(m.locMessage + "\n")
        }
    case wizardStepLogin:
        b.WriteString("First-time setup: login\n\n")
        b.WriteString("Save location: " + m.inputs[0].Value() + "\n\n")
        b.WriteString("Auto login (recommended): press 'a' to start capture.\n")
        if m.autoActive {
            b.WriteString(fmt.Sprintf("Listening on http://127.0.0.1:%d/capture\n", m.autoPort))
            b.WriteString("- Press 'o' to open fansly.com in your browser.\n")
            b.WriteString("- Press 'c' to copy the capture snippet to clipboard, then paste in DevTools Console and press Enter.\n")
            if m.snippet != "" {
                preview := m.snippet
                if len(preview) > 120 {
                    preview = preview[:120] + "..."
                }
                b.WriteString("Snippet preview: " + preview + "\n")
            }
            if m.waiting {
                b.WriteString("Waiting for browser to send details...\n")
            }
        } else {
            b.WriteString("(This will auto-fill User-Agent and Token below.)\n")
        }
        b.WriteString("\n")
        // Manual inputs still available
        b.WriteString(m.inputs[1].View() + "\n")
        b.WriteString(m.inputs[2].View() + "\n\n")
        if m.message != "" {
            b.WriteString(m.message + "\n")
        }
        b.WriteString("Press Enter to save, Tab to switch, Esc to quit.\n")
    }
    return b.String()
}

// Messages and commands for auto-capture flow
type autoCaptureStartedMsg struct {
    Port int
    Ch   <-chan auth.CapturedInfo
    Stop func(ctx context.Context) error
}

type autoCaptureResultMsg struct {
    UserAgent string
    Token     string
    Err       error
}

func (m *ConfigWizardModel) startAutoCaptureCmd() tea.Cmd {
    return func() tea.Msg {
        port, ch, stop, err := auth.StartAutoCaptureServer()
        if err != nil {
            return autoCaptureResultMsg{Err: err}
        }
        return autoCaptureStartedMsg{Port: port, Ch: ch, Stop: stop}
    }
}

func (m *ConfigWizardModel) waitForCaptureCmd(ch <-chan auth.CapturedInfo) tea.Cmd {
    return func() tea.Msg {
        select {
        case res, ok := <-ch:
            if !ok {
                return autoCaptureResultMsg{Err: fmt.Errorf("capture channel closed")}
            }
            return autoCaptureResultMsg{UserAgent: res.UserAgent, Token: res.Token}
        case <-time.After(5 * time.Minute):
            if m.stopFn != nil {
                _ = m.stopFn(context.Background())
                m.stopFn = nil
            }
            return autoCaptureResultMsg{Err: fmt.Errorf("timed out waiting for capture")}
        }
    }
}

func buildCaptureSnippet(port int) string {
    // Use an Image beacon to avoid CORS preflight; keep it short.
    return fmt.Sprintf(`(function(){try{var s=localStorage.getItem('session_active_session');var t=s?JSON.parse(s).token:'';var u=navigator.userAgent;var i=new Image();i.src='http://127.0.0.1:%d/capture?token='+encodeURIComponent(t)+'&ua='+encodeURIComponent(u)+'&ts='+Date.now();console.log('Sent token to scraper at localhost');}catch(e){console.log('Capture error',e);}})();`, port)
}

func openURL(url string) error {
    switch runtime.GOOS {
    case "windows":
        return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
    case "darwin":
        return exec.Command("open", url).Start()
    default:
        return exec.Command("xdg-open", url).Start()
    }
}
