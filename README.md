# 💩 PoopGo — AI Agent Harness

PoopGo is a terminal-based AI chat client built with [Bubble Tea v2](https://github.com/charmbracelet/bubbletea).
It streams responses from any OpenAI-compatible API right in your terminal.

## Features

- 🖥️ Full TUI with scrollable chat history (mouse wheel / touchpad, slash commands)
- 🖱️ Native text selection — drag to select, Cmd+C to copy
- ⚡ Streaming token-by-token responses (SSE) with animated spinner
- 💭 Reasoning model support — `reasoning_content` rendered in italic, `POOPGO_REASONING_EFFORT` config
- 🔌 Works with OpenAI, local LLMs (Ollama, LM Studio), or any `/chat/completions` endpoint
- ⌨️ Slash command palette (`/help`, `/scroll-top`, `/scroll-bottom`)
- 🧪 Fake provider for UI testing without API calls

## Quick Start

### 1. Set your API key

```bash
export POOPGO_API_KEY="sk-your-key-here"
```

If `POOPGO_API_KEY` is not set (and `POOPGO_PROVIDER` is not `fake`),
PoopGo prints an error to stderr and exits immediately.

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
| `POOPGO_REASONING_EFFORT` | *(empty → disabled)*  | Reasoning depth: `"low"`, `"medium"`, `"high"`, `"xhigh"`, `"max"` (for reasoning models like o1/o3) |
| `POOPGO_TEMPERATURE` | *(empty → API default)* | Sampling temperature `0.0`–`2.0` (e.g., `"0.7"`) |

All can be set via environment variables or a `.env` file:

```bash
# Option A: export individually
export POOPGO_API_KEY="sk-..."

# Option B: load from .env file (bash/zsh)
set -a; . ./.env; set +a
```

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

**DeepSeek (reasoning):**
```bash
export POOPGO_API_KEY="sk-..."
export POOPGO_BASE_URL="https://api.deepseek.com"
export POOPGO_MODEL="deepseek-reasoner"
export POOPGO_REASONING_EFFORT="max"
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

**With custom temperature:**
```bash
export POOPGO_API_KEY="sk-..."
export POOPGO_TEMPERATURE="0.7"
go run ./cmd/poopgo
```

## Keybindings

| Key              | Action                       |
|------------------|------------------------------|
| `Enter`          | Send message                 |
| `Shift+Enter`    | Insert newline               |
| `Ctrl+C`         | Quit                         |
| `Esc`            | Close palette (command mode) / no-op (normal mode) |
| `/`              | Open command palette         |
| `↑`/`↓` in palette | Navigate commands          |
| Mouse wheel / touchpad | Scroll (terminal-native) |
| Mouse drag       | Select text (terminal-native → Cmd+C) |

### Slash Commands

Type `/` at the start of a message to open the command palette:

| Command           | Description         |
|-------------------|---------------------|
| `/help`           | Show all commands   |
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
cmd/poopgo/main.go          Entry point — provider selection, Bubble Tea Program
internal/app/model.go       Main Model (viewport, textarea, messages, command palette, spinner)
internal/app/model_test.go  Model unit tests — keyboard, messages, streaming, command palette
internal/app/api.go         Types (Message, chatRequest) + SSE stream parsing
internal/app/api_test.go    SSE parsing + JSON serialization tests
internal/app/provider.go    StreamProvider interface + RealProvider + FakeProvider
```

## License

MIT — see [LICENSE](./LICENSE).
