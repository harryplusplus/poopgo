package app

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestModel() *Model {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "", "", NewFakeProvider())
	m.width = 100
	m.height = 30
	m.viewport.SetWidth(100)
	m.viewport.SetHeight(25) // 30 - 5
	m.refreshViewport()
	return m
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

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

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

// Regression test for #31: initErr removed from model; main.go now fails fast
// on missing API key with stderr + exit 1. The welcome screen must not
// show any initErr message.
func TestRefreshViewport_noInitErr(t *testing.T) {
	m := newTestModel()
	m.messages = nil
	m.refreshViewport()
	content := stripANSI(m.viewport.View())

	if strings.Contains(content, "POOPGO_API_KEY") {
		t.Errorf("welcome must not reference POOPGO_API_KEY after fail-fast change (#31): %s", content)
	}
	if strings.Contains(content, ".env") {
		t.Errorf("welcome must not reference .env: %s", content)
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

func TestStatusLine_reasoningEffortShown(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "high", "", NewFakeProvider())
	m.width = 100
	s := stripANSI(m.statusLine())
	if !strings.Contains(s, "💭high") {
		t.Errorf("status line missing reasoning effort indicator: %s", s)
	}
}

func TestStatusLine_reasoningEffortNotShownWhenEmpty(t *testing.T) {
	m := newTestModel()
	s := stripANSI(m.statusLine())
	if strings.Contains(s, "💭") {
		t.Errorf("reasoning indicator should not appear when empty: %s", s)
	}
}

func TestStatusLine_temperatureShown(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "", "0.7", NewFakeProvider())
	m.width = 100
	s := stripANSI(m.statusLine())
	if !strings.Contains(s, "🌡️0.7") {
		t.Errorf("status line missing temperature indicator: %s", s)
	}
}

func TestStatusLine_temperatureNotShownWhenEmpty(t *testing.T) {
	m := newTestModel()
	s := stripANSI(m.statusLine())
	if strings.Contains(s, "🌡️") {
		t.Errorf("temperature indicator should not appear when empty: %s", s)
	}
}

func TestStatusLine_bothShown(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "max", "0.7", NewFakeProvider())
	m.width = 100
	s := stripANSI(m.statusLine())
	if !strings.Contains(s, "💭max") {
		t.Errorf("status line missing reasoning effort: %s", s)
	}
	if !strings.Contains(s, "🌡️0.7") {
		t.Errorf("status line missing temperature: %s", s)
	}
}

// ---------------------------------------------------------------------------
// Model Update
// ---------------------------------------------------------------------------

func TestUpdate_windowSize(t *testing.T) {
	m := newTestModel()
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	m2, _ := m.Update(msg)

	updated := m2.(*Model)
	if updated.width != 120 {
		t.Errorf("width = %d", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height = %d", updated.height)
	}
	if updated.viewport.Width() != 120 {
		t.Errorf("viewport width = %d", updated.viewport.Width())
	}
	if updated.viewport.Height() != 35 {
		t.Errorf("viewport height = %d, want 35 (40-5)", updated.viewport.Height())
	}
}

func TestUpdate_quitOnCtrlC(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}

func TestUpdate_escNoQuitInNormalMode(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("esc in normal mode should not quit; only Ctrl+C quits")
	}
}

func TestUpdate_escClosesCommandMode(t *testing.T) {
	m := newTestModel()
	// Enter command mode
	m.textarea.SetValue("/")
	m.updateCommandMode()
	if !m.commandMode {
		t.Fatal("should be in command mode")
	}
	// Esc should exit command mode but not quit
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.commandMode {
		t.Error("should exit command mode on esc")
	}
	if cmd != nil {
		t.Fatal("esc in command mode should not quit")
	}
}

func TestUpdate_enterEmptyIgnored(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("   ")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no command for empty input")
	}
	if m.streaming {
		t.Error("should not be streaming after empty enter")
	}
}

func TestUpdate_enterWithMissingKey(t *testing.T) {
	m := newTestModel()
	m.apiKey = ""
	m.textarea.SetValue("hello")
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.textarea.Value() != "" {
		t.Error("textarea should be reset")
	}
	if len(m.messages) != 1 || m.messages[0].Role != "system" {
		t.Errorf("expected 1 system message, got %d: %+v", len(m.messages), m.messages)
	}
}

func TestUpdate_enterSubmitsMessage(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

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
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("expected no command when streaming")
	}
	if len(m.messages) != 0 {
		t.Error("messages should not change while streaming")
	}
}

func TestUpdate_streamChunkMsg(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: ""},
	}
	m.streaming = true

	_, _ = m.Update(StreamChunkMsg("Hello"))
	_, _ = m.Update(StreamChunkMsg(" world"))

	if m.messages[1].Content != "Hello world" {
		t.Errorf("assistant content: %q", m.messages[1].Content)
	}
}

func TestUpdate_streamChunkMsg_noAssistantSlot(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(StreamChunkMsg("orphan"))
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

	_, cmd := m.Update(StreamDoneMsg{})
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

	_, _ = m.Update(StreamDoneMsg{Err: &testError{"something broke"}})

	if m.streaming {
		t.Error("streaming flag should be cleared after error")
	}
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

// ---------------------------------------------------------------------------
// Spinner
// ---------------------------------------------------------------------------

func TestSpinner_tickCommandOnEnter(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected spinner tick command on enter")
	}
	// Verify the command produces a TickMsg
	msg := cmd()
	if _, ok := msg.(spinner.TickMsg); !ok {
		t.Errorf("expected spinner.TickMsg, got %T", msg)
	}
}

func TestSpinner_tickContinuesWhileStreaming(t *testing.T) {
	m := newTestModel()
	m.streaming = true

	_, cmd := m.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Error("expected tick command while streaming")
	}
}

func TestSpinner_tickStopsWhenNotStreaming(t *testing.T) {
	m := newTestModel()
	m.streaming = false

	_, cmd := m.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Error("expected no tick command when not streaming")
	}
}

func TestSpinner_stopsOnStreamDoneMsg(t *testing.T) {
	m := newTestModel()
	m.streaming = true

	// After StreamDoneMsg, streaming should be false
	_, _ = m.Update(StreamDoneMsg{})

	// Subsequent tick should not produce a command
	_, cmd := m.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Error("expected no tick command after StreamDoneMsg")
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestView_loadingState(t *testing.T) {
	m := newTestModel()
	m.width = 0
	s := m.View().Content
	if !strings.Contains(s, "Loading") {
		t.Errorf("expected Loading, got %s", s)
	}
}

func TestView_normalState(t *testing.T) {
	m := newTestModel()
	s := m.View().Content
	if !strings.Contains(s, "Ctrl+C quit") {
		t.Errorf("missing status bar: %s", s)
	}
}

// ---------------------------------------------------------------------------
// appendSystem
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Keyboard input flow
// ---------------------------------------------------------------------------

func TestTextareaStartsFocused(t *testing.T) {
	m := newTestModel()
	if !m.textarea.Focused() {
		t.Error("textarea should be focused initially")
	}
}

func TestTypeCharacters(t *testing.T) {
	m := newTestModel()

	// Type 'h'
	_, _ = m.Update(tea.KeyPressMsg{Text: "h"})
	if m.textarea.Value() != "h" {
		t.Errorf("value = %q, want %q", m.textarea.Value(), "h")
	}

	// Type 'i'
	_, _ = m.Update(tea.KeyPressMsg{Text: "i"})
	if m.textarea.Value() != "hi" {
		t.Errorf("value = %q, want %q", m.textarea.Value(), "hi")
	}
}

func TestBackspace(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello")

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.textarea.Value() != "hell" {
		t.Errorf("value = %q, want %q", m.textarea.Value(), "hell")
	}
}

func TestShiftEnterInsertsNewline(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("line1")

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if m.textarea.Value() != "line1\n" {
		t.Errorf("value = %q, want %q", m.textarea.Value(), "line1\n")
	}
}

func TestShiftEnterBlockedWhileStreaming(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.textarea.SetValue("line1")

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	_ = cmd
	if m.textarea.Value() != "line1" {
		t.Errorf("value should not change while streaming, got %q", m.textarea.Value())
	}
}

func TestCommandModeActivates(t *testing.T) {
	m := newTestModel()

	// Type '/'
	_, _ = m.Update(tea.KeyPressMsg{Text: "/"})
	if !m.commandMode {
		t.Error("command mode should activate on '/'")
	}
	if m.textarea.Value() != "/" {
		t.Errorf("value = %q, want %q", m.textarea.Value(), "/")
	}

	// Type 'h' → value becomes "/h", still starts with /
	_, _ = m.Update(tea.KeyPressMsg{Text: "h"})
	if !m.commandMode {
		t.Error("command mode should stay active for '/h'")
	}
	if len(m.filteredCommands) == 0 {
		t.Error("filteredCommands should not be empty")
	}

	// Clear the '/' → command mode deactivates
	m.textarea.SetValue("hello")
	_, _ = m.Update(tea.KeyPressMsg{Text: "!"})
	if m.commandMode {
		t.Error("command mode should deactivate when value doesn't start with '/'")
	}

	// Re-type '/' + 'h' + 'e' + 'l' + 'p' → command mode, filter narrows
	m.textarea.SetValue("/help")
	_, _ = m.Update(tea.KeyPressMsg{Text: "x"})
	// Value is "/helpx", still starts with /
	if !m.commandMode {
		t.Error("command mode should stay active for /helpx")
	}
	if len(m.filteredCommands) == 0 {
		t.Error("filteredCommands should fall back to all commands when no match")
	}
}

func TestCommandModeExecutesHelp(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("/help")
	m.updateCommandMode() // populate filteredCommands

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = cmd

	if m.commandMode {
		t.Error("command mode should exit after executing")
	}
	if m.textarea.Value() != "" {
		t.Error("textarea should be reset after command execution")
	}
	hasHelp := false
	for _, msg := range m.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Available commands") {
			hasHelp = true
		}
	}
	if !hasHelp {
		t.Error("expected /help output in system message")
	}
}

func TestTypingNotBlocked(t *testing.T) {
	// End-to-end: type message, submit, verify
	m := newTestModel()

	// Type message character by character
	for _, r := range "hello world" {
		_, _ = m.Update(tea.KeyPressMsg{Text: string(r)})
	}

	if m.textarea.Value() != "hello world" {
		t.Errorf("value = %q, want %q", m.textarea.Value(), "hello world")
	}

	// Submit
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.textarea.Value() != "" {
		t.Error("textarea should be cleared after submit")
	}
	if !m.streaming {
		t.Error("should be streaming after submit")
	}
	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}
}

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

// ---------------------------------------------------------------------------
// Reasoning
// ---------------------------------------------------------------------------

func TestUpdate_streamReasoningMsg(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "", ReasoningContent: ""},
	}
	m.streaming = true

	_, _ = m.Update(StreamReasoningMsg("Let me"))
	_, _ = m.Update(StreamReasoningMsg(" think"))

	if m.messages[1].ReasoningContent != "Let me think" {
		t.Errorf("reasoning content: %q", m.messages[1].ReasoningContent)
	}
	if m.messages[1].Content != "" {
		t.Errorf("content should still be empty, got %q", m.messages[1].Content)
	}
}

func TestUpdate_streamReasoningMsg_noAssistantSlot(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(StreamReasoningMsg("orphan"))
	_ = cmd
	if len(m.messages) != 0 {
		t.Errorf("messages should still be empty, got %d", len(m.messages))
	}
}

func TestRefreshViewport_reasoningRendered(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "Hello", ReasoningContent: "Let me think about this"},
	}
	m.refreshViewport()
	content := m.viewport.View()

	// Reasoning header and content should be present
	if !strings.Contains(content, "💭 Reasoning") {
		t.Error("missing reasoning header")
	}
	if !strings.Contains(content, "Let me think about this") {
		t.Error("missing reasoning content")
	}
	// Regular content still rendered
	if !strings.Contains(content, "Hello") {
		t.Error("missing regular content")
	}
	// Italic ANSI escapes should be present (\033[3m ... \033[23m)
	if !strings.Contains(content, "\033[3m") {
		t.Error("missing italic-on escape")
	}
	if !strings.Contains(content, "\033[23m") {
		t.Error("missing italic-off escape")
	}
}

func TestRefreshViewport_noReasoningWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "Hello", ReasoningContent: ""},
	}
	m.refreshViewport()
	content := m.viewport.View()

	if strings.Contains(content, "💭 Reasoning") {
		t.Error("reasoning header should not appear when ReasoningContent is empty")
	}
}

func TestNewModel_reasoningEffortStored(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "high", "", NewFakeProvider())
	if m.reasoningEffort != "high" {
		t.Errorf("reasoningEffort = %q, want %q", m.reasoningEffort, "high")
	}
}

func TestNewModel_reasoningEffort_xhigh(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "xhigh", "", NewFakeProvider())
	if m.reasoningEffort != "xhigh" {
		t.Errorf("reasoningEffort = %q, want %q", m.reasoningEffort, "xhigh")
	}
}

func TestNewModel_reasoningEffort_max(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "max", "", NewFakeProvider())
	if m.reasoningEffort != "max" {
		t.Errorf("reasoningEffort = %q, want %q", m.reasoningEffort, "max")
	}
}

func TestNewModel_reasoningEffortEmptyByDefault(t *testing.T) {
	m := newTestModel()
	if m.reasoningEffort != "" {
		t.Errorf("reasoningEffort should be empty, got %q", m.reasoningEffort)
	}
}

// ---------------------------------------------------------------------------
// Temperature
// ---------------------------------------------------------------------------

func TestNewModel_temperatureStored(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "", "0.7", NewFakeProvider())
	if m.temperature != "0.7" {
		t.Errorf("temperature = %q, want %q", m.temperature, "0.7")
	}
}

func TestNewModel_temperatureEmptyByDefault(t *testing.T) {
	m := newTestModel()
	if m.temperature != "" {
		t.Errorf("temperature should be empty by default, got %q", m.temperature)
	}
}

// ---------------------------------------------------------------------------
// Viewport soft wrap (issue #20)
// ---------------------------------------------------------------------------

func TestViewport_softWrapEnabled(t *testing.T) {
	m := newTestModel()
	if !m.viewport.SoftWrap {
		t.Error("viewport SoftWrap should be enabled by default")
	}
}

func TestViewport_longLineWraps(t *testing.T) {
	m := newTestModel()
	// Set a narrow viewport
	m.viewport.SetWidth(20)
	m.viewport.SetHeight(10)

	// Content with a line much wider than the viewport
	longLine := "this is a very long line that should wrap to multiple lines in the viewport"
	m.messages = []Message{
		{Role: "assistant", Content: longLine},
	}
	m.refreshViewport()

	// With SoftWrap enabled, TotalLineCount should be > 1 (the long line wraps)
	totalLines := m.viewport.TotalLineCount()
	if totalLines <= 1 {
		t.Errorf("expected wrapped lines > 1, got totalLineCount=%d (content=%q)", totalLines, longLine)
	}

	// The full text should still appear in the viewport content (not truncated)
	rawContent := m.viewport.GetContent()
	if !strings.Contains(rawContent, "wrap to multiple lines") {
		t.Error("content appears truncated; long line fragment missing")
	}
}

func TestViewport_longLineVisible(t *testing.T) {
	m := newTestModel()
	m.viewport.SetWidth(30)
	m.viewport.SetHeight(15)

	longLine := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	m.messages = []Message{
		{Role: "user", Content: longLine},
	}
	m.refreshViewport()

	view := m.viewport.View()
	// With SoftWrap, we should see portions of the long line in the rendered output
	// The entire string may be split across lines, but some portion must be visible
	if !strings.Contains(view, "abcdefghij") {
		t.Error("viewport View() doesn't contain the beginning of the long line")
	}
	// The end of the string should also be reachable (via scrolling or wrapping)
	rawContent := m.viewport.GetContent()
	if !strings.Contains(rawContent, "UVWXYZ") {
		t.Error("end of long line is truncated from content")
	}
}

func TestFakeProvider_temperatureEchoed(t *testing.T) {
	m := NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "", "0.7", NewFakeProvider())
	m.width = 100
	m.height = 30
	m.textarea.SetValue("hello")
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Simulate full fake provider stream synchronously (no goroutine needed)
	// Reconstruct what streamResponse does — call provider.Stream directly.
	var contentTokens []string
	err := m.provider.Stream(
		[]Message{{Role: "user", Content: "hello"}},
		"gpt-4o",
		func(tok string) { contentTokens = append(contentTokens, tok) },
		nil,
		"",
		"0.7",
	)
	if err != nil {
		t.Fatalf("fake provider stream: %v", err)
	}
	full := strings.Join(contentTokens, "")
	if !strings.Contains(full, "🌡️") {
		t.Error("expected temperature indicator in fake provider output")
	}
	if !strings.Contains(full, "0.7") {
		t.Error("expected temperature value 0.7 in fake provider output")
	}
}

// ---------------------------------------------------------------------------
// Command palette visual distinction (issue #22)
// ---------------------------------------------------------------------------

func TestCommandPalette_visualDistinction(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("/")
	m.updateCommandMode()

	palette := m.renderCommandPalette()

	// Palette must have a background color to visually distinguish from
	// the viewport content (issue #22).
	// lipgloss Background("236") produces ANSI 256-color escape \033[48;5;236m.
	if !strings.Contains(palette, "\033[48;5;236") {
		t.Error("palette missing background color (issue #22): should visually distinguish from viewport")
	}

	// Header must be present
	if !strings.Contains(palette, "Commands") {
		t.Error("palette missing header")
	}

	// Selected item must have a distinct background (not the same as panel bg).
	// lipgloss Background("8") + Foreground("15") → \033[97;100m (bright white on bright black).
	if !strings.Contains(palette, "\033[97;100m") {
		t.Error("selected item should have distinct background from panel")
	}
}

func TestCommandPalette_inViewOutput(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("/")
	m.updateCommandMode()

	view := m.View().Content

	// The palette should appear in the View output when command mode is active
	if !strings.Contains(view, "\033[48;5;236") {
		t.Error("View should include palette with background when command mode is active")
	}
	if !strings.Contains(view, "Commands") {
		t.Error("View should include palette header when command mode is active")
	}
}

func TestFakeProvider_temperatureNotEchoedWhenEmpty(t *testing.T) {
	m := newTestModel()
	var contentTokens []string
	err := m.provider.Stream(
		[]Message{{Role: "user", Content: "hello"}},
		"gpt-4o",
		func(tok string) { contentTokens = append(contentTokens, tok) },
		nil,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("fake provider stream: %v", err)
	}
	full := strings.Join(contentTokens, "")
	if strings.Contains(full, "🌡️") {
		t.Error("temperature indicator should not appear when temperature is empty")
	}
}

// ---------------------------------------------------------------------------
// Command palette layout — viewport height adjusts (issue #33)
// ---------------------------------------------------------------------------

func TestViewportHeight_normalMode(t *testing.T) {
	m := newTestModel()
	// Default: height=30, overhead=5 → viewport height = 25
	if m.viewport.Height() != 25 {
		t.Errorf("viewport height should be 25 (30-5), got %d", m.viewport.Height())
	}
}

func TestViewportHeight_commandModeShrinksViewport(t *testing.T) {
	m := newTestModel()
	// Activate command mode
	m.textarea.SetValue("/")
	m.updateCommandMode()

	// Palette = header(1) + 5 commands + footer(1) = 7 extra lines
	// height=30, overhead=5+7=12 → viewport height = 18
	if m.viewport.Height() != 18 {
		t.Errorf("viewport height should be 18 (30-5-7), got %d", m.viewport.Height())
	}
	if !m.commandMode {
		t.Error("should be in command mode")
	}
}

func TestViewportHeight_commandModeRestoresOnExit(t *testing.T) {
	m := newTestModel()
	// Enter command mode
	m.textarea.SetValue("/")
	m.updateCommandMode()

	// Exit command mode
	m.exitCommandMode()

	if m.viewport.Height() != 25 {
		t.Errorf("viewport height should restore to 25 (30-5) after exiting command mode, got %d", m.viewport.Height())
	}
	if m.commandMode {
		t.Error("should not be in command mode")
	}
}

func TestViewportHeight_commandModeEscRestoresHeight(t *testing.T) {
	m := newTestModel()
	// Activate command mode via keypress
	m.textarea.SetValue("/")
	m.updateCommandMode()

	if m.viewport.Height() != 18 {
		t.Errorf("viewport height should be 18 in command mode, got %d", m.viewport.Height())
	}

	// Esc should exit command mode and restore height
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

	if m.viewport.Height() != 25 {
		t.Errorf("viewport height should restore to 25 after Esc, got %d", m.viewport.Height())
	}
}

func TestViewportHeight_commandModeCtrlCRestoresHeight(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("/")
	m.updateCommandMode()

	// Ctrl+C in command mode exits palette but doesn't quit
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	_ = cmd

	if m.viewport.Height() != 25 {
		t.Errorf("viewport height should restore to 25 after Ctrl+C in command mode, got %d", m.viewport.Height())
	}
	if m.commandMode {
		t.Error("should exit command mode on ctrl+c")
	}
}

func TestViewportHeight_windowSizeInCommandMode(t *testing.T) {
	m := newTestModel()
	// Enter command mode first
	m.textarea.SetValue("/")
	m.updateCommandMode()

	// Now simulate a window resize while in command mode
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})

	// Palette overhead = 5 + 7 = 12, viewport height = 50 - 12 = 38
	if m.viewport.Height() != 38 {
		t.Errorf("viewport height should be 38 (50-12) in command mode after resize, got %d", m.viewport.Height())
	}
	if m.width != 120 {
		t.Errorf("width should be 120, got %d", m.width)
	}
}

func TestViewportHeight_minimumHeight(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 10 // very small terminal

	// Enter command mode — overhead would be 5+7=12 > 10
	m.textarea.SetValue("/")
	m.updateCommandMode()

	// Viewport height should be at least 1, not negative
	if m.viewport.Height() < 1 {
		t.Errorf("viewport height should be at least 1, got %d", m.viewport.Height())
	}
}

