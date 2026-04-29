package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// StreamProvider abstracts the LLM streaming API call.
// It sends messages to an LLM, invokes onToken for each content delta,
// onReasoningToken for each reasoning_content delta (reasoning models only),
// and onToolCall for each tool_calls delta (function calling).
// reasoningEffort controls reasoning depth ("low", "medium", "high"); empty means disabled.
// temperature sets the sampling temperature (0.0-2.0); empty means API default.
// ctx allows the caller to cancel the stream (e.g., on Esc keypress).
type StreamProvider interface {
	Stream(ctx context.Context, messages []Message, model string, onToken, onReasoningToken func(string), onToolCall func(index int, id, name, argsChunk string), reasoningEffort, temperature string) error
}

// ---------------------------------------------------------------------------
// RealProvider — real HTTP API calls
// ---------------------------------------------------------------------------

// RealProvider makes real HTTP requests to an OpenAI-compatible /chat/completions API.
type RealProvider struct {
	apiKey  string
	baseURL string
}

// NewRealProvider creates a RealProvider with the given credentials.
func NewRealProvider(apiKey, baseURL string) *RealProvider {
	return &RealProvider{apiKey: apiKey, baseURL: baseURL}
}

// Stream implements StreamProvider by POSTing to the chat completions endpoint
// and parsing the SSE response stream.
func (p *RealProvider) Stream(ctx context.Context, messages []Message, model string, onToken, onReasoningToken func(string), onToolCall func(index int, id, name, argsChunk string), reasoningEffort, temperature string) error {
	payload := chatRequest{
		Model:           model,
		Messages:        messages,
		Stream:          true,
		ReasoningEffort: reasoningEffort,
	}
	if temperature != "" {
		t, err := strconv.ParseFloat(temperature, 32)
		if err != nil {
			return fmt.Errorf("invalid POOPGO_TEMPERATURE %q: %w", temperature, err)
		}
		t32 := float32(t)
		payload.Temperature = &t32
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(p.baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(errBody))
	}

	return parseSSEStream(resp.Body, onToken, onReasoningToken, onToolCall)
}

// ---------------------------------------------------------------------------
// FakeProvider — fake provider for testing without API calls
// ---------------------------------------------------------------------------

// FakeProvider is a fake StreamProvider that returns canned responses.
// No HTTP calls are made; useful for testing the TUI without an API key or network.
type FakeProvider struct{}

// NewFakeProvider creates a FakeProvider.
func NewFakeProvider() *FakeProvider {
	return &FakeProvider{}
}

// Stream implements StreamProvider by echoing the last user message with a
// fake-provider banner. If reasoningEffort is set, emits fake reasoning tokens
// first. Temperature is echoed but not applied. Each character is emitted as
// a separate token to exercise the streaming path.
// onToolCall is accepted but ignored (FakeProvider does not emit tool calls).
func (p *FakeProvider) Stream(ctx context.Context, messages []Message, model string, onToken, onReasoningToken func(string), onToolCall func(index int, id, name, argsChunk string), reasoningEffort, temperature string) error {
	// Emit fake reasoning if reasoningEffort is set
	if reasoningEffort != "" && onReasoningToken != nil {
		reasoning := "🤔 [FAKE REASONING] Thinking with effort=" + reasoningEffort + "... "
		for _, ch := range reasoning {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			onReasoningToken(string(ch))
		}
	}

	echo := "🧪 [FAKE PROVIDER] "
	if len(messages) > 0 {
		echo += "Echo: " + messages[len(messages)-1].Content
	} else {
		echo += "No input."
	}
	echo += "\n\nPOOPGO_PROVIDER=fake → using fake provider (no API calls)."
	if temperature != "" {
		echo += fmt.Sprintf("\n🌡️  Temperature = %s", temperature)
	}

	for _, ch := range echo {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		onToken(string(ch))
	}
	return nil
}
