package app

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	// Italic ANSI escape should be present (ansi.Style{}.Italic(true) → \033[3m...\033[m)
	if !strings.Contains(content, "\033[3m") {
		t.Error("missing italic-on escape")
	}
	// ansi.Style uses SGR reset (\033[m), not specific italic-off (\033[23m)
	if !strings.Contains(content, "\033[m") {
		t.Error("missing SGR reset escape")
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

	// Palette = header(1) + 3 commands + footer(1) = 5 extra lines
	// height=30, overhead=5+5=10 → viewport height = 20
	if m.viewport.Height() != 20 {
		t.Errorf("viewport height should be 20 (30-5-5), got %d", m.viewport.Height())
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

	if m.viewport.Height() != 20 {
		t.Errorf("viewport height should be 20 in command mode, got %d", m.viewport.Height())
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

	// Palette overhead = 5 + 5 = 10, viewport height = 50 - 10 = 40
	if m.viewport.Height() != 40 {
		t.Errorf("viewport height should be 40 (50-10) in command mode after resize, got %d", m.viewport.Height())
	}
	if m.width != 120 {
		t.Errorf("width should be 120, got %d", m.width)
	}
}

// ---------------------------------------------------------------------------
// Paragraph spacing regression (issue #32)
// ---------------------------------------------------------------------------

func TestRefreshViewport_noDoubleSpacingBetweenMessages(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you?"},
		{Role: "assistant", Content: "great"},
	}
	m.refreshViewport()
	// GetContent() returns the raw set content without viewport View() line padding.
	content := m.viewport.GetContent()

	// Between messages there should be exactly 1 blank line, not 2+.
	// Before the fix, each message had a leading \n before the role header,
	// causing trailing \n + leading \n + role \n = \n\n\n potential triple.
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("paragraph spacing regression (#32): found triple newline, messages have double spacing\ncontent dump:\n%s", content)
	}
}

func TestRefreshViewport_paragraphsInMessage(t *testing.T) {
	m := newTestModel()
	// Simulate an assistant message with multi-paragraph content (like real LLM output)
	m.messages = []Message{
		{Role: "user", Content: "explain async/await"},
		{Role: "assistant", Content: "Async/await is a pattern for handling asynchronous operations.\n\nIt makes async code look synchronous.\n\nThis avoids callback hell."},
	}
	m.refreshViewport()
	// GetContent() returns the raw content string set into the viewport, before
	// line padding. Use this (not View()) to inspect paragraph breaks.
	content := m.viewport.GetContent()

	// The paragraphs from the LLM (with \n\n) should be preserved
	if !strings.Contains(content, "handling asynchronous operations.\n\nIt makes") {
		t.Errorf("multi-paragraph message content not preserved correctly (#32)\ncontent dump:\n%s", content)
	}

	// But there should be no triple-newline between the role header and content.
	// Before the fix, the leading \n before each role header caused double
	// spacing: previous msg trailing \n + leading \n + role \n = potential \n\n\n.
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("triple newlines found — paragraph spacing regression (#32)\ncontent dump:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Reasoning paragraph spacing regression (issue #18)
// ---------------------------------------------------------------------------

// TestLipglossV2_RenderDestroysParagraphBreaks proves the root cause of #18.
// lipgloss v2 Style.Render() pads each line of multi-line content to the
// maximum line width, replacing empty lines ("") with space-padded lines.
// This destroys \n\n paragraph break patterns.
func TestLipglossV2_RenderDestroysParagraphBreaks(t *testing.T) {
	// Simulate what sysStyle.Render(reasoning) does
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	reasoning := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."

	rendered := style.Render(reasoning)

	// With lipgloss v2, empty lines become space-padded → \n\n is destroyed.
	// After stripping ANSI, empty lines still contain the padding spaces.
	raw := stripANSI(rendered)
	if strings.Contains(rendered, "\n\n") {
		t.Errorf("lipgloss v2 unexpectedly preserved \\n\\n — upstream may have fixed padding")
	}
	// Even after ANSI strip, the padding spaces remain — so \n\n is still gone.
	if strings.Contains(raw, "\n\n") {
		t.Errorf("ANSI-stripped rendered unexpectedly has \\n\\n — padding may have changed")
	}

	// Verify empty line became space-padded (len > 2 = ANSI codes only)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	// lines[1] is the first empty line. After stripping ANSI, it should still have
	// spaces (the padding). If it's empty, lipgloss behavior has changed.
	plainEmpty := stripANSI(lines[1])
	if plainEmpty == "" {
		t.Log("WARNING: lipgloss v2 no longer pads empty lines — upstream fix?")
	} else {
		t.Logf("CONFIRMED: lipgloss v2 padded empty line to %d chars: %q", len(plainEmpty), plainEmpty)
	}

	// Plain text (our fix) preserves \n\n
	if !strings.Contains(reasoning, "\n\n") {
		t.Error("plain reasoning should always have \\n\\n")
	}
}

// TestRefreshViewport_reasoningExcessiveNewlines reproduces the bug where
// reasoning content from the model contains many consecutive blank lines
// between chain-of-thought paragraphs. Those should be collapsed to at most
// one blank line between paragraphs (\n\n).
func TestRefreshViewport_reasoningExcessiveNewlines(t *testing.T) {
	m := newTestModel()
	// Reasoning content with 6 consecutive newlines between paragraphs
	// (simulating the actual reasoning model output pattern)
	m.messages = []Message{
		{Role: "user", Content: "안녕하십니까요?"},
		{Role: "assistant", Content: "안녕하세요!", ReasoningContent: "We need to parse the user's query.\n\n\n\n\n\nWe need to respond appropriately.\n\n\n\n\n\nThe user's command: We are in a chat interface."},
	}
	m.refreshViewport()
	// Strip ANSI to inspect raw newline structure (lipgloss wraps each line
	// with color codes which break up consecutive \n sequences in GetContent).
	content := stripANSI(m.viewport.GetContent())

	// Should have no quadruple+ newline sequences
	if strings.Contains(content, "\n\n\n\n") {
		t.Errorf("reasoning content has 4+ consecutive newlines — should be collapsed (#18)\ncontent dump:\n%s", content)
	}
	// Triple newline should also be absent (at most \n\n for one blank line)
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("reasoning content has triple newline — should be collapsed to at most \\n\\n (#18)\ncontent dump:\n%s", content)
	}
}

// TestRefreshViewport_reasoningNormalParagraphs ensures that normal paragraph
// breaks (\n\n) in reasoning content are preserved (not over-collapsed).
func TestRefreshViewport_reasoningNormalParagraphs(t *testing.T) {
	m := newTestModel()
	m.messages = []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "response", ReasoningContent: "First thought.\n\nSecond thought.\n\nThird thought."},
	}
	m.refreshViewport()
	content := stripANSI(m.viewport.GetContent())

	// Normal paragraph breaks (\n\n) should be preserved
	if !strings.Contains(content, "First thought.\n\nSecond thought.") {
		t.Errorf("normal paragraph breaks in reasoning should be preserved (#18)\ncontent dump:\n%s", content)
	}
	// But no triple newlines
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("triple newlines in reasoning content — regression (#18)\ncontent dump:\n%s", content)
	}
}

// TestRefreshViewport_reasoningNewlines_DontAffectContent ensures that the
// newline normalization only applies to reasoning content, not the regular
// assistant message content.
func TestRefreshViewport_reasoningNewlines_DontAffectContent(t *testing.T) {
	m := newTestModel()
	// Regular content has paragraph breaks — those should be preserved as-is
	m.messages = []Message{
		{Role: "user", Content: "explain"},
		{Role: "assistant", Content: "Here is a detailed explanation.\n\nIt has multiple paragraphs.\n\nEach with normal spacing.", ReasoningContent: "Let me think.\n\n\n\n\nAbout this."},
	}
	m.refreshViewport()
	content := stripANSI(m.viewport.GetContent())

	// Content paragraph breaks (\n\n) should be preserved
	if !strings.Contains(content, "detailed explanation.\n\nIt has") {
		t.Errorf("regular content paragraph breaks should be preserved (#18)\ncontent dump:\n%s", content)
	}
	// Reasoning excessive newlines should be collapsed
	if strings.Contains(content, "Let me think.\n\n\n") {
		t.Errorf("reasoning excessive newlines should be collapsed (#18)\ncontent dump:\n%s", content)
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

// ---------------------------------------------------------------------------
// Keyboard scrolling (issue #27 — native text selection)
// ---------------------------------------------------------------------------

// TestKeyboardScrollUp scrolls viewport up by 1 line in normal mode.
func TestKeyboardScrollUp(t *testing.T) {
	m := newTestModel()
	m.viewport.SetHeight(10)
	// Add enough messages to fill the viewport and scroll
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{Role: "user", Content: fmt.Sprintf("line %d", i)})
	}
	m.refreshViewport()
	m.viewport.GotoBottom() // start at bottom

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	// After pressing Up, viewport should have scrolled up
	if m.viewport.AtBottom() {
		t.Error("viewport should have scrolled up after pressing Up")
	}
}

// TestKeyboardScrollDown scrolls viewport down by 1 line in normal mode.
func TestKeyboardScrollDown(t *testing.T) {
	m := newTestModel()
	m.viewport.SetHeight(10)
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{Role: "user", Content: fmt.Sprintf("line %d", i)})
	}
	m.refreshViewport()
	m.viewport.GotoTop() // start at top

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.viewport.AtTop() {
		t.Error("viewport should have scrolled down after pressing Down")
	}
}

// TestKeyboardPageUp removed: /scroll-up removed (Ghostty handles scrolling).
// TestKeyboardPageDown removed: /scroll-down removed (Ghostty handles scrolling).

// TestKeyboardScrollInCommandMode passes Up/Down through to command palette navigation.
func TestKeyboardScrollInCommandMode(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("/")
	m.updateCommandMode()

	// In command mode, Down should move selection, not scroll viewport
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selectedCmd != 1 {
		t.Errorf("command mode: Down should move selection to 1, got %d", m.selectedCmd)
	}

	// Up should move selection back
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selectedCmd != 0 {
		t.Errorf("command mode: Up should move selection to 0, got %d", m.selectedCmd)
	}
}

// TestViewMouseModeNone verifies that the View returns MouseModeNone for
// native text selection support (issue #27).
func TestViewMouseModeNone(t *testing.T) {
	m := newTestModel()
	v := m.View()
	// MouseMode should be None (0), not CellMotion (1).
	// When mouse mode is off, the terminal handles native text selection.
	if v.MouseMode != tea.MouseModeNone {
		t.Errorf("MouseMode should be None (native text selection), got %v", v.MouseMode)
	}
}

// TestStatusLineShowsScrollHelp verifies the status line indicates
// keyboard scrolling availability.
func TestStatusLineShowsScrollHelp(t *testing.T) {
	m := newTestModel()
	s := stripANSI(m.statusLine())
	if !strings.Contains(s, "scroll") {
		t.Errorf("status line should mention scroll help: %s", s)
	}
	// Should not mention mouse wheel since mouse mode is off
	if strings.Contains(strings.ToLower(s), "mouse") || strings.Contains(strings.ToLower(s), "wheel") {
		t.Errorf("status line should not mention mouse wheel (mouse mode off): %s", s)
	}
}

// TestUpDownNotBlockType prevents regression: Up/Down scroll the viewport
// but typing regular characters still flows into the textarea.
func TestKeyboardScrollDoesNotBlockTyping(t *testing.T) {
	m := newTestModel()

	// Scroll up first
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	// Then type — should reach textarea
	_, _ = m.Update(tea.KeyPressMsg{Text: "h"})
	if m.textarea.Value() != "h" {
		t.Errorf("typing after scrolling: expected 'h', got %q", m.textarea.Value())
	}
}

// ---------------------------------------------------------------------------
// Streaming with reasoning — end-to-end simulation (issue #18)
// ---------------------------------------------------------------------------

// endToEndStreamReasoning creates a Model with a FakeProvider, feeds a user
// message via Enter, and then simulates the full streaming flow synchronously
// by calling provider.Stream() with callbacks that feed Update(). Returns the
// final viewport content (ANSI-stripped).
func endToEndStreamReasoning(t *testing.T, userMsg string, reasoningEffort string) string {
	t.Helper()
	m := newTestModel()
	m.reasoningEffort = reasoningEffort
	m.textarea.SetValue(userMsg)

	// Simulate Enter - this appends user msg + empty assistant, sets streaming=true
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !m.streaming {
		t.Fatal("expected streaming after Enter")
	}
	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}

	// Simulate the streaming goroutine synchronously, feeding each token
	// through the Model's Update method (same path as real TUI).
	nonStreamingMessages := m.messages[:len(m.messages)-1]
	onToken := func(token string) {
		_, _ = m.Update(StreamChunkMsg(token))
	}
	onReasoningToken := func(token string) {
		_, _ = m.Update(StreamReasoningMsg(token))
	}
	err := m.provider.Stream(nonStreamingMessages, m.chatModel, onToken, onReasoningToken, m.reasoningEffort, m.temperature)
	if err != nil {
		t.Fatalf("provider.Stream: %v", err)
	}
	_, _ = m.Update(StreamDoneMsg{})

	return stripANSI(m.viewport.GetContent())
}

// TestStreamingReasoning_noExcessiveNewlines verifies that during a complete
// streaming flow with reasoning content (FakeProvider emits reasoning when
// reasoningEffort is set), the viewport does not accumulate excessive blank
// lines between reasoning paragraphs.
func TestStreamingReasoning_noExcessiveNewlines(t *testing.T) {
	content := endToEndStreamReasoning(t, "hello world", "high")

	// FakeProvider with reasoningEffort="high" emits reasoning tokens like:
	// "🤔 [FAKE REASONING] Thinking with effort=high... "
	// (no paragraph breaks in fake output, but we verify the rendering path)

	// Content should have no triple+ newlines anywhere
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("excessive newlines in viewport after full reasoning stream (#18)\ncontent dump:\n%s", content)
	}

	// Reasoning header should be present
	if !strings.Contains(content, "💭 Reasoning") {
		t.Error("missing reasoning header after streaming")
	}

	// Fake reasoning content should be present
	if !strings.Contains(content, "FAKE REASONING") {
		t.Error("missing fake reasoning content after streaming")
	}

	// Regular fake provider echo should also be present
	if !strings.Contains(content, "FAKE PROVIDER") {
		t.Error("missing fake provider echo after streaming")
	}
}

// TestStreamingReasoning_excessiveNewlinesCollapsed uses a custom provider
// that emits reasoning content with many consecutive newlines between
// paragraphs (simulating real DeepSeek/OpenAI reasoning model output).
// Verifies that the viewport collapses these to at most one blank line.
func TestStreamingReasoning_excessiveNewlinesCollapsed(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello")

	// Simulate Enter to create user + empty assistant
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}

	// Custom inline provider that emits reasoning with many newlines
	reasoningParts := []string{
		"Let me analyze this.",
		"\n\n\n\n\n\n", // 6 consecutive newlines (simulates model output)
		"After consideration,",
		"\n\n\n\n\n\n",
		"I think the answer is clear.",
	}
	contentParts := []string{
		"Here is my response.",
	}

	onToken := func(token string) {
		_, _ = m.Update(StreamChunkMsg(token))
	}
	onReasoningToken := func(token string) {
		_, _ = m.Update(StreamReasoningMsg(token))
	}

	// Emit reasoning tokens character by character to simulate streaming
	for _, part := range reasoningParts {
		for _, ch := range part {
			onReasoningToken(string(ch))
		}
	}
	for _, part := range contentParts {
		for _, ch := range part {
			onToken(string(ch))
		}
	}

	_, _ = m.Update(StreamDoneMsg{})

	content := stripANSI(m.viewport.GetContent())

	// Reasoning content should be collapsed: no triple+ newlines
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("reasoning content with 6x newlines was not collapsed — found triple+ newlines (#18)\ncontent dump:\n%s", content)
	}

	// But the reasoning paragraphs should still be separated by one blank line
	if !strings.Contains(content, "Let me analyze this.\n\nAfter consideration,") {
		t.Errorf("collapsed reasoning paragraphs missing expected \\n\\n separator (#18)\ncontent dump:\n%s", content)
	}

	// Content should be intact
	if !strings.Contains(content, "Here is my response.") {
		t.Error("regular content missing after streaming")
	}

	// Verify streaming flag was cleared
	if m.streaming {
		t.Error("streaming should be false after StreamDoneMsg")
	}
}

// TestKeyboardScrollWhileStreaming still scrolls (streaming doesn't block
// viewport interaction).
func TestKeyboardScrollWhileStreaming(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.viewport.SetHeight(10)
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{Role: "user", Content: fmt.Sprintf("line %d", i)})
	}
	m.refreshViewport()
	m.viewport.GotoBottom()

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.viewport.AtBottom() {
		t.Error("viewport should scroll even while streaming")
	}
}

