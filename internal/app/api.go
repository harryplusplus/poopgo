package app

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
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

// StreamChunkMsg carries a single token from the LLM stream.
type StreamChunkMsg string

// StreamDoneMsg signals that the LLM stream has finished (or errored).
type StreamDoneMsg struct{ Err error }

// ---------------------------------------------------------------------------
// SSE parsing
// ---------------------------------------------------------------------------

// parseSSEStream reads an SSE (Server-Sent Events) stream from r and calls
// onToken for each content delta found in the "data:" lines.
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
