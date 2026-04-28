package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	userStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	aiStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	sysStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the top-level Bubble Tea model for the PoopGo TUI.
type Model struct {
	program *tea.Program // injected after NewProgram to allow goroutines to Send()

	viewport viewport.Model
	textarea textarea.Model

	// Chat state
	messages     []Message
	assistantBuf string
	streaming    bool

	// Config
	apiKey    string
	apiBase   string
	chatModel string

	// Layout
	width  int
	height int

	// Error to display once
	initErr string
}

// NewModel creates a Model with the given configuration.  Call SetProgram
// after tea.NewProgram to enable streaming.
func NewModel(apiKey, apiBase, chatModel, initErr string) *Model {
	ta := textarea.New()
	ta.Placeholder = "Message… (Enter to send, Shift+Enter for newline)"
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Foreground(lipgloss.Color("12"))
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)
	vp.KeyMap = viewport.KeyMap{}

	m := &Model{
		viewport:  vp,
		textarea:  ta,
		apiKey:    apiKey,
		apiBase:   apiBase,
		chatModel: chatModel,
		initErr:   initErr,
		messages:  make([]Message, 0),
	}
	m.refreshViewport()
	return m
}

// SetProgram injects the *tea.Program reference needed for asynchronous
// streaming via p.Send().  Must be called after tea.NewProgram.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - 4
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			if m.streaming {
				break
			}
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				break
			}
			if m.apiKey == "" {
				m.appendSystem("⚠️  POOPGO_API_KEY is not set.")
				m.textarea.Reset()
				return m, nil
			}

			m.messages = append(m.messages, Message{Role: "user", Content: input})
			m.textarea.Reset()
			m.textarea.Blur()

			m.messages = append(m.messages, Message{Role: "assistant", Content: ""})
			m.assistantBuf = ""
			m.streaming = true
			m.refreshViewport()
			m.viewport.GotoBottom()

			go m.streamResponse()
			return m, nil

		case "shift+enter":
			if !m.streaming {
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}

		default:
			if !m.streaming {
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case StreamChunkMsg:
		n := len(m.messages)
		if n > 0 && m.messages[n-1].Role == "assistant" {
			m.messages[n-1].Content += string(msg)
			m.refreshViewport()
			m.viewport.GotoBottom()
		}

	case StreamDoneMsg:
		m.streaming = false
		m.textarea.Focus()
		if msg.Err != nil {
			m.appendSystem(fmt.Sprintf("❌ Error: %v", msg.Err))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()
		cmds = append(cmds, textarea.Blink)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	sep := strings.Repeat("─", m.width)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(sep)
	status := m.statusLine()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		sepStyle,
		m.textarea.View(),
		status,
	)
}

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------

func (m *Model) statusLine() string {
	left := fmt.Sprintf(" %s | %s", m.chatModel, m.apiBase)
	if m.streaming {
		left += " ● streaming"
	}
	right := "Ctrl+C quit"
	width := m.width
	if width < len(left)+len(right)+2 {
		width = len(left) + len(right) + 2
	}
	pad := width - len(left) - len(right)
	if pad < 1 {
		pad = 1
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render(left + strings.Repeat(" ", pad) + right)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m *Model) refreshViewport() {
	var sb strings.Builder

	if len(m.messages) == 0 {
		sb.WriteString("🤖 PoopGo — AI Agent Harness\n\n")
		if m.initErr != "" {
			sb.WriteString("⚠️  " + m.initErr + "\n\n")
		}
		sb.WriteString("Enter a message to start.\n")
	}

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			sb.WriteString("\n")
			sb.WriteString(userStyle.Render("🧑 You"))
			sb.WriteString("\n")
			sb.WriteString(msg.Content + "\n")
		case "assistant":
			sb.WriteString("\n")
			sb.WriteString(aiStyle.Render("🤖 AI"))
			sb.WriteString("\n")
			sb.WriteString(msg.Content + "\n")
		case "system":
			sb.WriteString("\n")
			sb.WriteString(sysStyle.Render(msg.Content))
			sb.WriteString("\n")
		}
	}

	m.viewport.SetContent(sb.String())
}

func (m *Model) appendSystem(text string) {
	m.messages = append(m.messages, Message{Role: "system", Content: text})
}

func (m *Model) streamResponse() {
	payload := chatRequest{
		Model:    m.chatModel,
		Messages: m.messages[:len(m.messages)-1],
		Stream:   true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		m.program.Send(StreamDoneMsg{Err: fmt.Errorf("marshal request: %w", err)})
		return
	}

	url := strings.TrimRight(m.apiBase, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		m.program.Send(StreamDoneMsg{Err: fmt.Errorf("create request: %w", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		m.program.Send(StreamDoneMsg{Err: fmt.Errorf("http request: %w", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		m.program.Send(StreamDoneMsg{
			Err: fmt.Errorf("API returned %d: %s", resp.StatusCode, string(errBody)),
		})
		return
	}

	onToken := func(token string) {
		m.program.Send(StreamChunkMsg(token))
	}
	if err := parseSSEStream(resp.Body, onToken); err != nil {
		m.program.Send(StreamDoneMsg{Err: fmt.Errorf("read stream: %w", err)})
		return
	}

	m.program.Send(StreamDoneMsg{})
}
