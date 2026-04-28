# AGENTS

## Hindsight Bank ID
poopgo

## Project
PoopGo — Bubble Tea 기반 터미널 AI 채팅 클라이언트.
OpenAI 호환 `/chat/completions` API와 SSE 스트리밍으로 동작.

## Commands
- `go build ./...`        빌드
- `go run ./cmd/poopgo`   실행 (fake provider: `POOPGO_PROVIDER=fake go run ./cmd/poopgo`)
- `go test ./internal/...` 테스트 (네트워크 불필요, 모든 테스트가 self-contained)
- `go vet ./...`          정적 분석

## Structure
| File | Role |
|------|------|
| `cmd/poopgo/main.go` | 진입점 — godotenv 로드, provider 선택, Bubble Tea Program 생성 |
| `internal/app/model.go` | 메인 Model — viewport, textarea, messages, command palette, Update/View |
| `internal/app/model_test.go` | Model 단위 테스트 — 키 입력, 메시지 흐름, 스트리밍, 커맨드 팔레트 |
| `internal/app/api.go` | 타입 정의 (Message, chatRequest, streamChunk) + SSE 파싱 |
| `internal/app/api_test.go` | SSE 파싱 + JSON 직렬화 테스트 |
| `internal/app/provider.go` | StreamProvider 인터페이스 + RealProvider + FakeProvider |

## Runtime Config
| Variable | Default | Description |
|---|---|---|
| `POOPGO_API_KEY` | *(필수, fake 제외)* | API 키 |
| `POOPGO_BASE_URL` | `https://api.openai.com/v1` | Chat completions API base URL |
| `POOPGO_MODEL` | `gpt-4o` | 모델명 |
| `POOPGO_PROVIDER` | *(empty → RealProvider)* | `"fake"` → FakeProvider |

`.env` 파일로도 설정 가능. `main.go`에서 `godotenv.Load()` 호출.

## Key Patterns

### Provider Architecture
- `StreamProvider` interface: `Stream(messages []Message, model string, onToken func(string)) error`
- `RealProvider`: OpenAI 호환 `/chat/completions` API에 HTTP POST → SSE 파싱
- `FakeProvider`: 마지막 user message를 echo + banner. API 호출 없음, 테스트용
- `main.go`에서 `POOPGO_PROVIDER` env var에 따라 provider 선택
- Model은 provider만 바라보고, HTTP/SSE 디테일을 모름 (의존성 역전)

### Model Lifecycle
- `NewModel()` 생성 후 `SetProgram(p *tea.Program)` 호출해야 goroutine에서 `p.Send()` 사용 가능
- `streamResponse()` → `provider.Stream(messages, model, onToken)` 호출. provider가 모든 HTTP/SSE 로직을 캡슐화
- `onToken` 콜백이 `p.Send(StreamChunkMsg(token))` 호출 → Model.Update에서 처리

### Key Handling
- `handleCommandMode()` — command mode 활성 시 `tea.KeyMsg` 인터셉터
- Command mode에서 `Esc`/`Ctrl+C`는 Quit 대신 palette만 닫기
- `Enter`: 메시지 전송 or command 실행
- `Alt+Enter`: textarea에 newline 삽입 (Bubble Tea v1.3.10에서 `Shift+Enter`는 별도 key type이 없으므로 `Alt+Enter` 사용)
- 그 외 일반 키: textarea로 전달 → `updateCommandMode()` 호출해 `/` prefix 감지
- Viewport KeyMap은 비어 있음 (`viewport.KeyMap{}`) → 키 스크롤 불가, 마우스 휠 또는 slash command로만 스크롤

### Streaming Flow
1. 사용자가 Enter → `messages`에 user + empty assistant 추가 → `streaming = true`
2. `go m.streamResponse()` → provider.Stream 호출
3. Provider가 토큰마다 `onToken` 콜백 → `p.Send(StreamChunkMsg)`
4. Model.Update의 `StreamChunkMsg` case에서 assistant message content 누적
5. 완료 시 `p.Send(StreamDoneMsg{Err: err})` → `streaming = false`, textarea 재포커스

### SSE Parsing
- `parseSSEStream()` — `data:` 라인에서 JSON content delta 추출
- `[DONE]`에서 종료
- Malformed JSON은 skip (에러 없음)
- Scanner buffer: 64KB initial, 1MB max

### Command Palette
- `/` 입력 → slash command palette 활성화
- Commands: `/help`, `/scroll-up`, `/scroll-down`, `/scroll-top`, `/scroll-bottom`
- `updateCommandMode()`가 textarea 값 기반으로 `filteredCommands` 필터링
- `executeCommand()`가 실제 동작 수행 (viewport 스크롤, system message 출력)

## Dependencies
| Module | Version | Usage |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | v1.3.10 | Elm 아키텍처 TUI 프레임워크 |
| `github.com/charmbracelet/bubbles` | v1.0.0 | textarea, viewport 컴포넌트 |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | 스타일링 |
| `github.com/joho/godotenv` | v1.5.1 | `.env` 로딩 |

## Known Constraints
- **Shift+Enter 미지원**: Bubble Tea v1.3.10에 `KeyShiftEnter` 타입이 없음. `Alt+Enter`로 newline 입력.
- **Viewport 키 스크롤 불가**: viewport.KeyMap이 비어 있어 화살표/PgUp/PgDn으로 채팅 스크롤 불가. 마우스 휠 또는 slash command(`/scroll-up`, `/scroll-down`) 사용.
- **TUI 테스트 한계**: `tea.Program.Run()`은 실제 터미널 필요. Model.Update에 keyMsg 직접 주입하는 방식으로 키 입력 테스트.

## Testing Guidelines
- 모든 테스트는 네트워크 불필요 (FakeProvider 사용)
- `newTestModel()` 헬퍼로 FakeProvider 기반 Model 생성
- `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}`로 문자 입력 시뮬레이션
- `tea.KeyMsg{Type: tea.KeyEnter}`로 Enter 시뮬레이션
- `tea.KeyMsg{Type: tea.KeyEnter, Alt: true}`로 Alt+Enter 시뮬레이션
- `StreamChunkMsg("token")`, `StreamDoneMsg{Err: err}`로 스트리밍 시뮬레이션
- `stripANSI()`로 ANSI 이스케이프 제거 후 문자열 검증
