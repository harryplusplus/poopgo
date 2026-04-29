package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commandItem is a single entry in the slash-command palette.
type commandItem struct {
	command     string // e.g. "/scroll-down"
	description string // e.g. "Page down"
}

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	userStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	aiStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	sysStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	italicOn        = "\033[3m"
	italicOff       = "\033[23m"
	reasoningHeader = "💭 Reasoning"
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
	apiKey          string
	apiBase         string
	chatModel       string
	reasoningEffort string
	temperature     string
	provider        StreamProvider

	// Layout
	width  int
	height int

	// Spinner shown during streaming
	spinner spinner.Model

	// Error to display once
	initErr string

	// Command palette (triggered by "/" at start)
	commandMode      bool
	commands         []commandItem
	filteredCommands []commandItem
	selectedCmd      int
}

var defaultCommands = []commandItem{
	{"/help", "Show available commands"},
	{"/scroll-up", "Page up"},
	{"/scroll-down", "Page down"},
	{"/scroll-top", "Scroll to top"},
	{"/scroll-bottom", "Scroll to bottom"},
}

// NewModel creates a Model with the given configuration.  Call SetProgram
// after tea.NewProgram to enable streaming.
func NewModel(apiKey, apiBase, chatModel, reasoningEffort, temperature, initErr string, provider StreamProvider) *Model {
	ta := textarea.New()
	ta.Placeholder = "Message… (/ for commands, Enter to send, Alt+Enter for newline)"
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Foreground(lipgloss.Color("12"))
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.Focus()

	vp := viewport.New(80, 20)
	vp.KeyMap = viewport.KeyMap{}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	m := &Model{
		viewport:        vp,
		textarea:        ta,
		spinner:         s,
		apiKey:          apiKey,
		apiBase:         apiBase,
		chatModel:       chatModel,
		reasoningEffort: reasoningEffort,
		temperature:     temperature,
		initErr:         initErr,
		provider:        provider,
		messages:        make([]Message, 0),
		commands:        defaultCommands,
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
		m.viewport.Height = msg.Height - 5 // separator(1) + textarea(3) + status(1)
		m.refreshViewport()

	case tea.KeyMsg:
		if m.handleCommandMode(msg) {
			m.refreshViewport()
			return m, nil
		}

		switch msg.String() {

		case "ctrl+c":
			if m.commandMode {
				m.exitCommandMode()
				m.refreshViewport()
				return m, nil
			}
			return m, tea.Quit

		case "esc":
			if m.commandMode {
				m.exitCommandMode()
				m.refreshViewport()
				return m, nil
			}
			// Esc in normal mode is a no-op — only Ctrl+C quits

		case "enter":
			if m.streaming {
				break
			}
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				break
			}
			if strings.HasPrefix(input, "/") {
				m.executeCommand(input)
				m.textarea.Reset()
				m.exitCommandMode()
				m.refreshViewport()
				return m, nil
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
			cmds = append(cmds, m.spinner.Tick)
			return m, tea.Batch(cmds...)

		case "alt+enter":
			if !m.streaming {
				m.textarea.SetValue(m.textarea.Value() + "\n")
				m.textarea, _ = m.textarea.Update(msg)
			}

		default:
			if !m.streaming {
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				// Check if textarea now starts with "/"
				m.updateCommandMode()
			}
		}

	case StreamChunkMsg:
		n := len(m.messages)
		if n > 0 && m.messages[n-1].Role == "assistant" {
			m.messages[n-1].Content += string(msg)
			m.refreshViewport()
			m.viewport.GotoBottom()
		}

	case StreamReasoningMsg:
		n := len(m.messages)
		if n > 0 && m.messages[n-1].Role == "assistant" {
			m.messages[n-1].ReasoningContent += string(msg)
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.streaming {
			cmds = append(cmds, cmd)
		}
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

	// In command mode, show palette above the separator
	palette := ""
	if m.commandMode {
		palette = m.renderCommandPalette() +
			"\n"
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		palette+sepStyle,
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
		left += " " + m.spinner.View() + " streaming"
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
			if msg.ReasoningContent != "" {
				sb.WriteString(sysStyle.Render(reasoningHeader))
				sb.WriteString("\n")
				sb.WriteString(italicOn)
				sb.WriteString(sysStyle.Render(msg.ReasoningContent))
				sb.WriteString(italicOff)
				sb.WriteString("\n")
			}
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

// ---------------------------------------------------------------------------
// Command palette
// ---------------------------------------------------------------------------

// handleCommandMode handles key events when the command palette is active.
// Returns true if the event was consumed.
func (m *Model) handleCommandMode(msg tea.KeyMsg) bool {
	if !m.commandMode {
		return false
	}

	switch msg.String() {
	case "esc", "ctrl+c":
		m.exitCommandMode()
		return true

	case "enter":
		// Execute selected command
		if len(m.filteredCommands) > 0 && m.selectedCmd < len(m.filteredCommands) {
			m.executeCommand(m.filteredCommands[m.selectedCmd].command)
			m.textarea.Reset()
			m.exitCommandMode()
		}
		return true

	case "up":
		if m.selectedCmd > 0 {
			m.selectedCmd--
		}
		return true

	case "down":
		if m.selectedCmd < len(m.filteredCommands)-1 {
			m.selectedCmd++
		}
		return true

	default:
		// Pass to textarea for typing, then update filter
		m.textarea, _ = m.textarea.Update(msg)
		m.updateCommandMode()
		return true
	}
}

// updateCommandMode checks the textarea value and updates the command palette.
func (m *Model) updateCommandMode() {
	val := m.textarea.Value()

	if !strings.HasPrefix(val, "/") {
		m.exitCommandMode()
		return
	}

	m.commandMode = true

	// Filter commands by prefix
	m.filteredCommands = nil
	m.selectedCmd = 0
	for _, c := range m.commands {
		if strings.HasPrefix(c.command, val) || strings.Contains(c.description, strings.TrimPrefix(val, "/")) {
			m.filteredCommands = append(m.filteredCommands, c)
		}
	}
	if len(m.filteredCommands) == 0 {
		m.filteredCommands = m.commands
	}
}

// exitCommandMode exits the command palette and resets state.
func (m *Model) exitCommandMode() {
	m.commandMode = false
	m.filteredCommands = nil
	m.selectedCmd = 0
	if strings.HasPrefix(m.textarea.Value(), "/") {
		m.textarea.Reset()
	}
}

// executeCommand runs a slash command locally.
func (m *Model) executeCommand(input string) {
	switch input {
	case "/help":
		var sb strings.Builder
		sb.WriteString("Available commands:\n")
		for _, c := range m.commands {
			sb.WriteString(fmt.Sprintf("  %-20s %s\n", c.command, c.description))
		}
		m.appendSystem(sb.String())

	case "/scroll-up":
		m.viewport.ViewUp()

	case "/scroll-down":
		m.viewport.ViewDown()

	case "/scroll-top":
		m.viewport.GotoTop()

	case "/scroll-bottom":
		m.viewport.GotoBottom()

	default:
		m.appendSystem(fmt.Sprintf("Unknown command: %s (type /help for commands)", input))
	}
}

// renderCommandPalette renders the command palette popup overlay.
func (m *Model) renderCommandPalette() string {
	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12"))
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15"))

	var sb strings.Builder
	sb.WriteString("  Commands:\n")

	for i, c := range m.filteredCommands {
		prefix := "  "
		if i == m.selectedCmd {
			prefix = "▸ "
			sb.WriteString(prefix)
			sb.WriteString(selectedStyle.Render(fmt.Sprintf("%-22s", c.command)))
			sb.WriteString(" ")
			sb.WriteString(selectedStyle.Render(c.description))
		} else {
			sb.WriteString(prefix)
			sb.WriteString(cmdStyle.Render(fmt.Sprintf("%-22s", c.command)))
			sb.WriteString(" ")
			sb.WriteString(descStyle.Render(c.description))
		}
		sb.WriteString("\n")
	}

	// Add a closing border line
	sb.WriteString("  ── Esc to close ──")

	return sb.String()
}

func (m *Model) streamResponse() {
	if m.program == nil {
		return
	}
	onToken := func(token string) {
		m.program.Send(StreamChunkMsg(token))
	}
	onReasoningToken := func(token string) {
		m.program.Send(StreamReasoningMsg(token))
	}
	err := m.provider.Stream(m.messages[:len(m.messages)-1], m.chatModel, onToken, onReasoningToken, m.reasoningEffort, m.temperature)
	m.program.Send(StreamDoneMsg{Err: err})
}
