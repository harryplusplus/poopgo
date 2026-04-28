# 💩 PoopGo — AI Agent Harness

PoopGo is a terminal-based AI chat client built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).  
It streams responses from any OpenAI-compatible API right in your terminal.

## Features

- 🖥️ Full TUI with scrollable chat history
- ⚡ Streaming token-by-token responses (SSE)
- 🔌 Works with OpenAI, local LLMs (Ollama, LM Studio), or any `/chat/completions` endpoint
- ⌨️ Vim-style and arrow key navigation

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

| Variable          | Default                        | Description                          |
|-------------------|--------------------------------|--------------------------------------|
| `POOPGO_API_KEY`  | *(required)*                   | Your API key                         |
| `POOPGO_BASE_URL` | `https://api.openai.com/v1`    | Base URL of the chat completions API |
| `POOPGO_MODEL`    | `gpt-4o`                       | Model name                           |

All three can be set via environment variables or a `.env` file.

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

## Keybindings

| Key           | Action               |
|---------------|----------------------|
| `Enter`       | Send message         |
| `Shift+Enter` | Newline              |
| `Esc` / `Ctrl+C` | Quit / Close palette|
| `↑` / `↓`     | Scroll chat history  |
| `PgUp` / `PgDn` | Page scroll (classic)|
| `/`           | Command palette      |

### Slash Commands

Type `/` at the start of a message to open the command palette:

| Command          | Description         |
|------------------|---------------------|
| `/help`          | Show all commands   |
| `/scroll-up`     | Page up             |
| `/scroll-down`   | Page down           |
| `/scroll-top`    | Scroll to top       |
| `/scroll-bottom` | Scroll to bottom    |

Use `↑`/`↓` to navigate, `Enter` to select, `Esc` to close.

## Running Tests

```bash
go test ./internal/...
```

## License

MIT — see [LICENSE](./LICENSE).
