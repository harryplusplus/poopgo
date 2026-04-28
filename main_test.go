package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestModel returns a model suitable for testing (no program, no env).
func newTestModel() *model {
	ta := textarea.New()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)
	vp.KeyMap = viewport.KeyMap{}

	m := &model{
		viewport:  vp,
		textarea:  ta,
		apiKey:    "sk-test",
		apiBase:   "https://api.openai.com/v1",
		chatModel: "gpt-4o",
		messages:  make([]Message, 0),
		width:     100,
		height:    30,
	}
	m.refreshViewport()
	return m
}

// word extracts the first word from a potentially styled string (skipping ANSI).
func word(s string) string {
	// lipgloss styles embed ANSI.  For tests we care about plain text, so
	// strip ansi and take first token.
	plain := stripANSI(s)
	f := strings.Fields(plain)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// SSE parsing
// ---------------------------------------------------------------------------

func TestParseSSEStream_singleChunk(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "Hello" {
		t.Fatalf("expected [Hello], got %v", tokens)
	}
}

func TestParseSSEStream_multipleChunks(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"He"}}]}` + "\n" +
		`data: {"choices":[{"delta":{"content":"llo"}}]}` + "\n" +
		`data: {"choices":[{"delta":{"content":" world"}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(tokens, "") != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", strings.Join(tokens, ""))
	}
}

func TestParseSSEStream_emptyDelta(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":""}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestParseSSEStream_malformedJSON(t *testing.T) {
	input := `data: {bad` + "\n" +
		`data: {"choices":[{"delta":{"content":"ok"}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "ok" {
		t.Fatalf("expected [ok], got %v", tokens)
	}
}

func TestParseSSEStream_noChoices(t *testing.T) {
	input := `data: {"choices":[]}` + "\n" +
		`data: [DONE]` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestParseSSEStream_noDone(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"x"}}]}` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "x" {
		t.Fatalf("expected [x], got %v", tokens)
	}
}

// ---------------------------------------------------------------------------
// JSON serialization
// ---------------------------------------------------------------------------

func TestChatRequestMarshal(t *testing.T) {
	req := chatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
		Stream: true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundtrip chatRequest
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundtrip.Model != "gpt-4o" {
		t.Errorf("model: %s", roundtrip.Model)
	}
	if len(roundtrip.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(roundtrip.Messages))
	}
	if roundtrip.Messages[0].Role != "user" || roundtrip.Messages[0].Content != "hi" {
		t.Errorf("message mismatch: %+v", roundtrip.Messages[0])
	}
	if !roundtrip.Stream {
		t.Error("stream should be true")
	}
}

func TestStreamChunkUnmarshal(t *testing.T) {
	raw := `{"choices":[{"delta":{"content":"hello"}}]}`
	var chunk streamChunk
	if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(chunk.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(chunk.Choices))
	}
	if chunk.Choices[0].Delta.Content != "hello" {
		t.Errorf("content: %s", chunk.Choices[0].Delta.Content)
	}
}

// ---------------------------------------------------------------------------
// refreshViewport / messages
// ---------------------------------------------------------------------------

func TestRefreshViewport_empty(t *testing.T) {
	m := newTestModel()
	content := m.viewport.View()

	if !strings.Contains(content, "PoopGo") {
		t.Errorf("viewport missing welcome: %s", content)
	}
}

func TestRefreshViewport_withMessages(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	m.refreshViewport()
	content := stripANSI(m.viewport.View())

	if !strings.Contains(content, "hello") {
		t.Errorf("missing user message: %s", content)
	}
	if !strings.Contains(content, "hi there") {
		t.Errorf("missing assistant message: %s", content)
	}
}

func TestRefreshViewport_systemMessage(t *testing.T) {
	m := newTestModel()
	m.appendSystem("an error occurred")
	m.refreshViewport()
	content := stripANSI(m.viewport.View())

	if !strings.Contains(content, "an error occurred") {
		t.Errorf("missing system message: %s", content)
	}
}

func TestRefreshViewport_initErr(t *testing.T) {
	m := newTestModel()
	m.initErr = "no api key"
	m.messages = nil
	m.refreshViewport()
	content := stripANSI(m.viewport.View())

	if !strings.Contains(content, "no api key") {
		t.Errorf("missing init error: %s", content)
	}
}

// ---------------------------------------------------------------------------
// statusLine
// ---------------------------------------------------------------------------

func TestStatusLine_idle(t *testing.T) {
	m := newTestModel()
	s := stripANSI(m.statusLine())
	if !strings.Contains(s, "gpt-4o") {
		t.Errorf("missing model: %s", s)
	}
	if strings.Contains(s, "streaming") {
		t.Errorf("should not say streaming when idle: %s", s)
	}
}

func TestStatusLine_streaming(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	s := stripANSI(m.statusLine())
	if !strings.Contains(s, "streaming") {
		t.Errorf("missing streaming indicator: %s", s)
	}
}

// ---------------------------------------------------------------------------
// Model Update
// ---------------------------------------------------------------------------

func TestUpdate_windowSize(t *testing.T) {
	m := newTestModel()
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	m2, _ := m.Update(msg)

	updated := m2.(*model)
	if updated.width != 120 {
		t.Errorf("width = %d", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height = %d", updated.height)
	}
	if updated.viewport.Width != 120 {
		t.Errorf("viewport width = %d", updated.viewport.Width)
	}
	if updated.viewport.Height != 36 { // height - 4
		t.Errorf("viewport height = %d", updated.viewport.Height)
	}
}

func TestUpdate_quitOnCtrlC(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
	// tea.Quit is a function; just check it's not nil.
}

func TestUpdate_quitOnEsc(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected quit command on esc")
	}
}

func TestUpdate_enterEmptyIgnored(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("   ") // whitespace only
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no command for empty input")
	}
	if m.streaming {
		t.Error("should not be streaming after empty enter")
	}
}

func TestUpdate_enterWithMissingKey(t *testing.T) {
	m := newTestModel()
	m.apiKey = "" // simulate missing key
	m.textarea.SetValue("hello")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.textarea.Value() != "" {
		t.Error("textarea should be reset")
	}
	// Should have a system message about missing key.
	if len(m.messages) != 1 || m.messages[0].Role != "system" {
		t.Errorf("expected 1 system message, got %d: %+v", len(m.messages), m.messages)
	}
}

func TestUpdate_enterSubmitsMessage(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.textarea.Value() != "" {
		t.Error("textarea should be cleared after submit")
	}
	if !m.streaming {
		t.Error("should be streaming after submit")
	}
	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}
	if m.messages[0].Role != "user" || m.messages[0].Content != "hello world" {
		t.Errorf("user message: %+v", m.messages[0])
	}
	if m.messages[1].Role != "assistant" || m.messages[1].Content != "" {
		t.Errorf("assistant placeholder: %+v", m.messages[1])
	}
}

func TestUpdate_enterBlockedWhileStreaming(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.textarea.SetValue("ignored")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("expected no command when streaming")
	}
	if len(m.messages) != 0 {
		t.Error("messages should not change while streaming")
	}
}

func TestUpdate_streamChunkMsg(t *testing.T) {
	m := newTestModel()
	// Simulate what happens after enter: 2 messages (user + empty assistant).
	m.messages = []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: ""},
	}
	m.streaming = true

	_, _ = m.Update(streamChunkMsg("Hello"))
	_, _ = m.Update(streamChunkMsg(" world"))

	if m.messages[1].Content != "Hello world" {
		t.Errorf("assistant content: %q", m.messages[1].Content)
	}
}

func TestUpdate_streamChunkMsg_noAssistantSlot(t *testing.T) {
	m := newTestModel()
	// Messages list is empty - chunk should be safely ignored.
	_, cmd := m.Update(streamChunkMsg("orphan"))
	_ = cmd
	if len(m.messages) != 0 {
		t.Errorf("messages should still be empty, got %d", len(m.messages))
	}
}

func TestUpdate_streamDoneMsg(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.messages = []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "Hello"},
	}

	_, cmd := m.Update(streamDoneMsg{})
	_ = cmd

	if m.streaming {
		t.Error("streaming flag should be cleared")
	}
}

func TestUpdate_streamDoneMsg_withError(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.messages = []Message{
		{Role: "user", Content: "hi"},
	}

	_, _ = m.Update(streamDoneMsg{err: &testError{"something broke"}})

	if m.streaming {
		t.Error("streaming flag should be cleared after error")
	}
	// Should have appended a system error message.
	hasErr := false
	for _, msg := range m.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "something broke") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected system error message, got: %+v", m.messages)
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestView_loadingState(t *testing.T) {
	m := newTestModel()
	m.width = 0
	s := m.View()
	if !strings.Contains(s, "Loading") {
		t.Errorf("expected Loading, got %s", s)
	}
}

func TestView_normalState(t *testing.T) {
	m := newTestModel()
	s := m.View()
	if !strings.Contains(s, "Ctrl+C quit") {
		t.Errorf("missing status bar: %s", s)
	}
}

// ---------------------------------------------------------------------------
// appendSystem
// ---------------------------------------------------------------------------

func TestAppendSystem(t *testing.T) {
	m := newTestModel()
	m.appendSystem("test message")

	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}
	if m.messages[0].Role != "system" {
		t.Errorf("role: %s", m.messages[0].Role)
	}
	if m.messages[0].Content != "test message" {
		t.Errorf("content: %s", m.messages[0].Content)
	}
}
