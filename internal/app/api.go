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
// ToolCalls is accumulated from tool_calls SSE deltas (function calling).
// ToolCallID is set for "tool" role messages carrying function results.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"-"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// ToolCall is a single tool call requested by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and JSON-encoded arguments.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool describes a function available to the model (OpenAI function calling).
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// chatRequest is the JSON body sent to the OpenAI-compatible /chat/completions endpoint.
type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Stream          bool      `json:"stream"`
	Temperature     *float32  `json:"temperature,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	Tools           []Tool    `json:"tools,omitempty"`
}

// streamChunk models a single SSE data payload from a streaming chat completion.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string            `json:"content"`
			ReasoningContent string            `json:"reasoning_content"`
			ToolCalls        []streamToolCall  `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// streamToolCall is a single tool call delta in an SSE chunk.
type streamToolCall struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function streamToolCallFunc   `json:"function"`
}

// streamToolCallFunc holds partial function call data from an SSE delta.
type streamToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// --- tea.Msg types for async streaming ---

// StreamChunkMsg carries a single content token from the LLM stream.
type StreamChunkMsg string

// StreamReasoningMsg carries a single reasoning token (reasoning models only).
type StreamReasoningMsg string

// StreamToolCallMsg carries a single tool_call delta from the LLM stream.
// Index identifies which tool call this fragment belongs to (multiple tool calls
// may be in flight simultaneously). ID and FunctionName are set on the first
// chunk; Arguments carries a JSON fragment to append.
type StreamToolCallMsg struct {
	Index        int
	ID           string
	FunctionName string
	Arguments    string
}

// StreamDoneMsg signals that the LLM stream has finished (or errored).
type StreamDoneMsg struct{ Err error }

// ---------------------------------------------------------------------------
// SSE parsing
// ---------------------------------------------------------------------------

// parseSSEStream reads an SSE (Server-Sent Events) stream from r and calls
// onToken for each content delta, onReasoningToken for each reasoning_content
// delta, and onToolCall for each tool_calls delta found in the "data:" lines.
// All callbacks may be nil.
func parseSSEStream(r io.Reader, onToken, onReasoningToken func(string), onToolCall func(index int, id, name, argsChunk string)) error {
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
			for _, tc := range delta.ToolCalls {
				if onToolCall != nil {
					onToolCall(tc.Index, tc.ID, tc.Function.Name, tc.Function.Arguments)
				}
			}
		}
	}

	return scanner.Err()
}
