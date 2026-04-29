# 💩 PoopGo — AI Agent Harness

PoopGo is a terminal-based AI chat client built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
It streams responses from any OpenAI-compatible API right in your terminal.

## Features

- 🖥️ Full TUI with scrollable chat history (mouse wheel, slash commands)
- ⚡ Streaming token-by-token responses (SSE) with animated spinner
- 💭 Reasoning model support — `reasoning_content` rendered in italic, `POOPGO_REASONING_EFFORT` config
- 🔌 Works with OpenAI, local LLMs (Ollama, LM Studio), or any `/chat/completions` endpoint
- ⌨️ Slash command palette (`/help`, `/scroll-up`, `/scroll-down`, …)
- 🧪 Fake provider for UI testing without API calls

## Quick Start

### 1. Set your API key

```bash
export POOPGO_API_KEY="sk-your-key-here"
```

Or create a `.env` file in the project root:

```
POOPGO_API_KEY=sk-your-key-here
```

### 2. Run

```bash
go run ./cmd/poopgo
```

### 3. (Optional) Build a binary

```bash
go build -o poopgo ./cmd/poopgo
./poopgo
```

## Configuration

| Variable           | Default                        | Description                          |
|--------------------|--------------------------------|--------------------------------------|
| `POOPGO_API_KEY`   | *(required, except fake)*      | Your API key                         |
| `POOPGO_BASE_URL`  | `https://api.openai.com/v1`    | Base URL of the chat completions API |
| `POOPGO_MODEL`     | `gpt-4o`                       | Model name                           |
| `POOPGO_PROVIDER`  | *(empty → real API)*           | `"fake"` for fake provider (no API)  |
| `POOPGO_REASONING_EFFORT` | *(empty → disabled)*  | Reasoning depth: `"low"`, `"medium"`, `"high"` (for reasoning models like o1/o3) |

All can be set via environment variables or a `.env` file.

### Examples

**OpenAI:**
```bash
export POOPGO_API_KEY="sk-..."
go run ./cmd/poopgo
```

**Ollama (local):**
```bash
export POOPGO_API_KEY="ollama"
export POOPGO_BASE_URL="http://localhost:11434/v1"
export POOPGO_MODEL="llama3"
go run ./cmd/poopgo
```

**LM Studio (local):**
```bash
export POOPGO_API_KEY="lm-studio"
export POOPGO_BASE_URL="http://localhost:1234/v1"
go run ./cmd/poopgo
```

**Fake provider (no API, no key):**
```bash
export POOPGO_PROVIDER="fake"
go run ./cmd/poopgo
```

**Reasoning model with fake provider:**
```bash
export POOPGO_PROVIDER="fake"
export POOPGO_REASONING_EFFORT="high"
go run ./cmd/poopgo
```

## Keybindings

| Key              | Action                       |
|------------------|------------------------------|
| `Enter`          | Send message                 |
| `Alt+Enter`      | Insert newline               |
| `Esc` / `Ctrl+C` | Quit (close palette in command mode) |
| `/`              | Open command palette         |
| Mouse wheel      | Scroll chat history          |
| Spinner in status | Appears while AI is responding |
| `↑`/`↓` in palette | Navigate commands         |

### Slash Commands

Type `/` at the start of a message to open the command palette:

| Command           | Description         |
|-------------------|---------------------|
| `/help`           | Show all commands   |
| `/scroll-up`      | Page up             |
| `/scroll-down`    | Page down           |
| `/scroll-top`     | Scroll to top       |
| `/scroll-bottom`  | Scroll to bottom    |

Use `↑`/`↓` to navigate, `Enter` to select, `Esc` to close.

## Testing

```bash
# Run all tests
go test ./internal/...

# Verbose output
go test ./internal/... -v

# With race detector
go test ./internal/... -race
```

### Test methodology

- **Unit tests** (`internal/app/model_test.go`): Model state transitions — keyboard input, message flow, streaming, command palette, viewport rendering.
- **API tests** (`internal/app/api_test.go`): SSE parsing, JSON serialization/deserialization.
- Use `POOPGO_PROVIDER=fake` for interactive testing without API calls.

All tests are self-contained; no network or external dependencies required.

## Project Structure

```
cmd/poopgo/main.go          Entry point — env loading, provider selection, Bubble Tea Program
internal/app/model.go       Main Model (viewport, textarea, messages, command palette, spinner)
internal/app/model_test.go  Model unit tests — keyboard, messages, streaming, command palette
internal/app/api.go         Types (Message, chatRequest) + SSE stream parsing
internal/app/api_test.go    SSE parsing + JSON serialization tests
internal/app/provider.go    StreamProvider interface + RealProvider + FakeProvider
```

## License

MIT — see [LICENSE](./LICENSE).
