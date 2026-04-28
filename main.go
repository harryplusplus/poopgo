package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Message is a single chat message exchanged with the LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the JSON body sent to the OpenAI-compatible /chat/completions endpoint.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// streamChunk models a single SSE data payload from a streaming chat completion.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// --- tea.Msg types for async streaming ---

// streamChunkMsg carries a single token from the LLM stream.
type streamChunkMsg string

// streamDoneMsg signals that the LLM stream has finished (or errored).
type streamDoneMsg struct{ err error }

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	program *tea.Program // injected after NewProgram to allow goroutines to Send()

	viewport viewport.Model
	textarea textarea.Model

	// Chat state
	messages     []Message
	assistantBuf string // accumulating assistant content during streaming
	streaming    bool

	// Config (from env / .env)
	apiKey    string
	apiBase   string
	chatModel string

	// Layout
	width  int
	height int

	// Error to display once
	initErr string
}

func initialModel() model {
	// Attempt .env load; failure is non-fatal.
	_ = godotenv.Load()

	apiKey := os.Getenv("POOPGO_API_KEY")
	apiBase := os.Getenv("POOPGO_API_BASE")
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	chatModel := os.Getenv("POOPGO_MODEL")
	if chatModel == "" {
		chatModel = "gpt-4o"
	}

	var initErr string
	if apiKey == "" {
		initErr = "POOPGO_API_KEY not set. Set it in your environment or .env file."
	}

	// Textarea: multi-line input at the bottom.
	ta := textarea.New()
	ta.Placeholder = "Message… (Enter to send, Shift+Enter for newline)"
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Foreground(lipgloss.Color("12"))
	// Disable Enter for newline so we can use it to submit.
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Viewport: scrollable chat history.
	vp := viewport.New(80, 20)
	vp.KeyMap = viewport.KeyMap{} // disable viewport default keybindings

	m := model{
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

func (m *model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// --- Window resize ---
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - 4 // 3 lines for textarea + 1 for separator gap
		m.refreshViewport()

	// --- Keyboard ---
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
			// Validate configuration before sending.
			if m.apiKey == "" {
				m.appendSystem("⚠️  POOPGO_API_KEY is not set.")
				m.textarea.Reset()
				return m, nil
			}

			// Append user message.
			m.messages = append(m.messages, Message{Role: "user", Content: input})
			m.textarea.Reset()
			m.textarea.Blur()

			// Reserve an empty assistant slot that will be filled by streaming.
			m.messages = append(m.messages, Message{Role: "assistant", Content: ""})
			m.assistantBuf = ""
			m.streaming = true
			m.refreshViewport()
			m.viewport.GotoBottom()

			// Kick off the LLM stream in a goroutine.
			go m.streamResponse()
			return m, nil

		case "shift+enter":
			// Let textarea handle newline insertion.
			if !m.streaming {
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}

		default:
			// Forward other keystrokes to textarea when not streaming.
			if !m.streaming {
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	// --- Streaming chunks ---
	case streamChunkMsg:
		n := len(m.messages)
		if n > 0 && m.messages[n-1].Role == "assistant" {
			m.messages[n-1].Content += string(msg)
			m.refreshViewport()
			m.viewport.GotoBottom()
		}

	// --- Stream finished ---
	case streamDoneMsg:
		m.streaming = false
		m.textarea.Focus()
		if msg.err != nil {
			m.appendSystem(fmt.Sprintf("❌ Error: %v", msg.err))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()
		cmds = append(cmds, textarea.Blink)
	}

	// Always let viewport process its own messages (e.g. mouse wheel).
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m *model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	// Separator line between viewport and textarea.
	sep := strings.Repeat("─", m.width)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(sep)

	// Status bar.
	status := m.statusLine()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		sepStyle,
		m.textarea.View(),
		status,
	)
}

func (m *model) statusLine() string {
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

// refreshViewport rebuilds the viewport content from the current messages.
func (m *model) refreshViewport() {
	var sb strings.Builder

	// Welcome / initial error.
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

func (m *model) appendSystem(text string) {
	m.messages = append(m.messages, Message{Role: "system", Content: text})
}

// streamResponse makes an HTTP streaming request to the LLM API and sends
// tokens back to the TUI event loop via program.Send().  This runs in a
// goroutine so it never blocks the Bubble Tea render loop.
func (m *model) streamResponse() {
	// Build the request body excluding the placeholder assistant message.
	payload := chatRequest{
		Model:    m.chatModel,
		Messages: m.messages[:len(m.messages)-1],
		Stream:   true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		m.program.Send(streamDoneMsg{err: fmt.Errorf("marshal request: %w", err)})
		return
	}

	url := strings.TrimRight(m.apiBase, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		m.program.Send(streamDoneMsg{err: fmt.Errorf("create request: %w", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		m.program.Send(streamDoneMsg{err: fmt.Errorf("http request: %w", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		m.program.Send(streamDoneMsg{
			err: fmt.Errorf("API returned %d: %s", resp.StatusCode, string(errBody)),
		})
		return
	}

	onToken := func(token string) {
		m.program.Send(streamChunkMsg(token))
	}
	if err := parseSSEStream(resp.Body, onToken); err != nil {
		m.program.Send(streamDoneMsg{err: fmt.Errorf("read stream: %w", err)})
		return
	}

	m.program.Send(streamDoneMsg{})
}

// parseSSEStream reads an SSE (Server-Sent Events) stream from r and calls
// onToken for each content delta found in the "data:" lines.  It is extracted
// for testability.
func parseSSEStream(r io.Reader, onToken func(string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onToken(chunk.Choices[0].Delta.Content)
		}
	}

	return scanner.Err()
}

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	userStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	aiStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	sysStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	m := initialModel()
	p := tea.NewProgram(
		&m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	// Inject program reference so goroutines can call p.Send().
	m.program = p

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "poopgo: %v\n", err)
		os.Exit(1)
	}
}
