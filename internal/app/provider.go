package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StreamProvider abstracts the LLM streaming API call.
// It sends messages to an LLM and invokes onToken for each content delta.
type StreamProvider interface {
	Stream(messages []Message, model string, onToken func(string)) error
}

// ---------------------------------------------------------------------------
// RealProvider — real HTTP API calls
// ---------------------------------------------------------------------------

// RealProvider makes real HTTP requests to an OpenAI-compatible /chat/completions API.
type RealProvider struct {
	apiKey  string
	apiBase string
}

// NewRealProvider creates a RealProvider with the given credentials.
func NewRealProvider(apiKey, apiBase string) *RealProvider {
	return &RealProvider{apiKey: apiKey, apiBase: apiBase}
}

// Stream implements StreamProvider by POSTing to the chat completions endpoint
// and parsing the SSE response stream.
func (p *RealProvider) Stream(messages []Message, model string, onToken func(string)) error {
	payload := chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(p.apiBase, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
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

	return parseSSEStream(resp.Body, onToken)
}

// ---------------------------------------------------------------------------
// TestProvider — fake provider for testing without API calls
// ---------------------------------------------------------------------------

// TestProvider is a fake StreamProvider that returns canned responses.
// No HTTP calls are made; useful for testing the TUI without an API key or network.
type TestProvider struct{}

// NewTestProvider creates a TestProvider.
func NewTestProvider() *TestProvider {
	return &TestProvider{}
}

// Stream implements StreamProvider by echoing the last user message with a
// test-provider banner. Each character is emitted as a separate token to
// exercise the streaming path.
func (p *TestProvider) Stream(messages []Message, model string, onToken func(string)) error {
	echo := "🧪 [TEST PROVIDER] "
	if len(messages) > 0 {
		echo += "Echo: " + messages[len(messages)-1].Content
	} else {
		echo += "No input."
	}
	echo += "\n\nPOOPGO_PROVIDER=test → using fake provider (no API calls)."

	for _, ch := range echo {
		onToken(string(ch))
	}
	return nil
}
