# AGENTS

## Hindsight Bank ID
poopgo

## Project
PoopGo — Bubble Tea 기반 터미널 AI 채팅 클라이언트.
OpenAI 호환 `/chat/completions` API와 SSE 스트리밍으로 동작.

## Commands
- `go build ./...`        빌드
- `go run ./cmd/poopgo`   실행
- `go test ./internal/...` 테스트

## Structure
- `cmd/poopgo/main.go`       진입점 — godotenv로 환경변수 로드, provider 선택, Bubble Tea Program 생성
- `internal/app/model.go`    메인 Model (viewport, textarea, messages, command palette)
- `internal/app/api.go`      타입 정의 (Message, chatRequest), SSE 스트리밍 파싱
- `internal/app/provider.go` StreamProvider 인터페이스 + RealProvider + TestProvider

## Runtime Config
| Variable           | Default                     |
|--------------------|-----------------------------|
| `POOPGO_API_KEY`   | *(필수, test provider는 예외)* |
| `POOPGO_BASE_URL`  | `https://api.openai.com/v1` |
| `POOPGO_MODEL`     | `gpt-4o`                    |
| `POOPGO_PROVIDER`  | *(empty → RealProvider)*    |

`.env` 파일로도 설정 가능.

`POOPGO_PROVIDER=test` → TestProvider 사용. API 호출 없이 마지막 메시지를 echo.
그 외 값이거나 미설정 → RealProvider (실제 HTTP API 호출).

## Key Patterns
- `Model` 생성 후 `SetProgram()` 호출해야 goroutine에서 `p.Send()` 사용 가능
- `streamResponse()` → `provider.Stream(messages, model, onToken)` 호출. provider가 모든 HTTP/SSE 로직을 캡슐화
- `handleCommandMode()` — `tea.KeyMsg` 인터셉터. command mode에서 `Esc`/`Ctrl+C`는 Quit 대신 palette만 닫기
- `/` 입력 → slash command palette: `/help`, `/scroll-up`, `/scroll-down`, `/scroll-top`, `/scroll-bottom`
- `parseSSEStream()` — SSE `data:` 라인에서 content delta 추출, `[DONE]`에서 종료

## Provider Architecture
- `StreamProvider` interface: `Stream(messages, model, onToken) error`
- `RealProvider`: OpenAI 호환 `/chat/completions` API에 HTTP POST → SSE 파싱
- `TestProvider`: fake — 마지막 user message를 echo, API 호출 없음
- `main.go`에서 `POOPGO_PROVIDER` env var에 따라 provider 선택
- Model은 provider만 바라보고, HTTP/SSE 디테일을 모름

## Dependencies
- `github.com/charmbracelet/bubbletea` — Elm 아키텍처 TUI 프레임워크
- `github.com/charmbracelet/bubbles` — textarea, viewport 컴포넌트
- `github.com/charmbracelet/lipgloss` — 스타일링
- `github.com/joho/godotenv` — `.env` 로딩
