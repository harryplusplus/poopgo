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
// ReasoningContent is accumulated from reasoning_content SSE deltas
// (reasoning models only). It is not serialized to the API request.
type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"-"`
}

// chatRequest is the JSON body sent to the OpenAI-compatible /chat/completions endpoint.
type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Stream          bool      `json:"stream"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

// streamChunk models a single SSE data payload from a streaming chat completion.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

// --- tea.Msg types for async streaming ---

// StreamChunkMsg carries a single content token from the LLM stream.
type StreamChunkMsg string

// StreamReasoningMsg carries a single reasoning token (reasoning models only).
type StreamReasoningMsg string

// StreamDoneMsg signals that the LLM stream has finished (or errored).
type StreamDoneMsg struct{ Err error }

// ---------------------------------------------------------------------------
// SSE parsing
// ---------------------------------------------------------------------------

// parseSSEStream reads an SSE (Server-Sent Events) stream from r and calls
// onToken for each content delta and onReasoningToken for each reasoning_content
// delta found in the "data:" lines. Either callback may be nil.
func parseSSEStream(r io.Reader, onToken, onReasoningToken func(string)) error {
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
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" && onToken != nil {
				onToken(delta.Content)
			}
			if delta.ReasoningContent != "" && onReasoningToken != nil {
				onReasoningToken(delta.ReasoningContent)
			}
		}
	}

	return scanner.Err()
}
