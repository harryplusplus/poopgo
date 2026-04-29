package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SSE parsing
// ---------------------------------------------------------------------------

func TestParseSSEStream_singleChunk(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var tokens []string
	err := parseSSEStream(strings.NewReader(input), func(tok string) {
		tokens = append(tokens, tok)
	}, nil)

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
	}, nil)

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
	}, nil)

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
	}, nil)

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
	}, nil)

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
	}, nil)

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
// Reasoning SSE parsing
// ---------------------------------------------------------------------------

func TestParseSSEStream_reasoningContent(t *testing.T) {
	input := `data: {"choices":[{"delta":{"reasoning_content":"Let me think"}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var reasoningTokens []string
	err := parseSSEStream(strings.NewReader(input), nil, func(tok string) {
		reasoningTokens = append(reasoningTokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(reasoningTokens, "") != "Let me think" {
		t.Fatalf("expected 'Let me think', got %q", strings.Join(reasoningTokens, ""))
	}
}

func TestParseSSEStream_contentAndReasoningInterleaved(t *testing.T) {
	input := `data: {"choices":[{"delta":{"reasoning_content":"Hmm"}}]}` + "\n" +
		`data: {"choices":[{"delta":{"content":"Hi"}}]}` + "\n" +
		`data: {"choices":[{"delta":{"reasoning_content":"..."}}]}` + "\n" +
		`data: {"choices":[{"delta":{"content":" there"}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var contentTokens []string
	var reasoningTokens []string
	err := parseSSEStream(strings.NewReader(input),
		func(tok string) { contentTokens = append(contentTokens, tok) },
		func(tok string) { reasoningTokens = append(reasoningTokens, tok) },
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(contentTokens, "") != "Hi there" {
		t.Errorf("content: %q", strings.Join(contentTokens, ""))
	}
	if strings.Join(reasoningTokens, "") != "Hmm..." {
		t.Errorf("reasoning: %q", strings.Join(reasoningTokens, ""))
	}
}

func TestParseSSEStream_reasoningEmptyIgnored(t *testing.T) {
	input := `data: {"choices":[{"delta":{"reasoning_content":""}}]}` + "\n" +
		`data: [DONE]` + "\n"

	var reasoningTokens []string
	err := parseSSEStream(strings.NewReader(input), nil, func(tok string) {
		reasoningTokens = append(reasoningTokens, tok)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reasoningTokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(reasoningTokens))
	}
}

// ---------------------------------------------------------------------------
// chatRequest with reasoning_effort
// ---------------------------------------------------------------------------

func TestChatRequestMarshalWithReasoningEffort(t *testing.T) {
	req := chatRequest{
		Model:           "o3-mini",
		Messages:        []Message{{Role: "user", Content: "hi"}},
		Stream:          true,
		ReasoningEffort: "high",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(data), `"reasoning_effort":"high"`) {
		t.Errorf("missing reasoning_effort in JSON: %s", string(data))
	}

	var roundtrip chatRequest
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundtrip.ReasoningEffort != "high" {
		t.Errorf("reasoning_effort = %q", roundtrip.ReasoningEffort)
	}
}

func TestChatRequestMarshalOmitemptyReasoningEffort(t *testing.T) {
	req := chatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "hi"}},
		Stream:   true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(data), "reasoning_effort") {
		t.Errorf("reasoning_effort should be omitted when empty: %s", string(data))
	}
}
