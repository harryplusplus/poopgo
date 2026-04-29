# AGENTS

## Hindsight Bank ID
poopgo

## Project
PoopGo — Bubble Tea 기반 터미널 AI 채팅 클라이언트.
OpenAI 호환 `/chat/completions` API와 SSE 스트리밍으로 동작.

## File Inventory
작업 시작 전에 두 파일을 모두 읽을 것:
- **`README.md`** — 사용자 대상 문서. 기능, 설정, 키바인딩, 실행 예제, 테스트 방법이 기술되어 있다. 프로젝트의 겉모습을 이해하는 데 필수.
- **`AGENTS.md`** (이 파일) — AI 작업자 대상 문서. 구조, 패턴, 제약사항, 테스트 가이드가 기술되어 있다. 코드 수정/작성 시 참고.

## Commands
- `go build ./...`        빌드
- `go run ./cmd/poopgo`   실행 (fake provider: `POOPGO_PROVIDER=fake go run ./cmd/poopgo`)
- `go test ./internal/...` 테스트 (네트워크 불필요, 모든 테스트가 self-contained)
- `go vet ./...`          정적 분석

## Structure
| File | Role |
|------|------|
| `cmd/poopgo/main.go` | 진입점 — godotenv 로드, provider 선택, Bubble Tea Program 생성 |
| `internal/app/model.go` | 메인 Model — viewport, textarea, messages, command palette, spinner, Update/View |
| `internal/app/model_test.go` | Model 단위 테스트 — 키 입력, 메시지 흐름, 스트리밍, 커맨드 팔레트 |
| `internal/app/api.go` | 타입 정의 (Message, chatRequest, streamChunk) + SSE 파싱 (content + reasoning_content) |
| `internal/app/api_test.go` | SSE 파싱 + JSON 직렬화 테스트 |
| `internal/app/provider.go` | StreamProvider 인터페이스 + RealProvider + FakeProvider |

## Runtime Config
| Variable | Default | Description |
|---|---|---|
| `POOPGO_API_KEY` | *(필수, fake 제외)* | API 키 |
| `POOPGO_BASE_URL` | `https://api.openai.com/v1` | Chat completions API base URL |
| `POOPGO_MODEL` | `gpt-4o` | 모델명 |
| `POOPGO_PROVIDER` | *(empty → RealProvider)* | `"fake"` → FakeProvider |
| `POOPGO_REASONING_EFFORT` | *(empty → disabled)* | Reasoning depth: `"low"`, `"medium"`, `"high"` (reasoning models only) |

`.env` 파일로도 설정 가능. `main.go`에서 `godotenv.Load()` 호출.

## Key Patterns

### Provider Architecture
- `StreamProvider` interface: `Stream(messages []Message, model string, onToken, onReasoningToken func(string), reasoningEffort string) error`
- `RealProvider`: OpenAI 호환 `/chat/completions` API에 HTTP POST → SSE 파싱
- `FakeProvider`: 마지막 user message를 echo + banner. `reasoningEffort`가 설정되면 가짜 reasoning 토큰을 먼저 emit. API 호출 없음, 테스트용
- `main.go`에서 `POOPGO_PROVIDER` env var에 따라 provider 선택
- `reasoningEffort`는 `main.go` → `NewModel` → `streamResponse()` → `provider.Stream()`으로 전달
- Model은 provider만 바라보고, HTTP/SSE 디테일을 모름 (의존성 역전)

### Model Lifecycle
- `NewModel()` 생성 후 `SetProgram(p *tea.Program)` 호출해야 goroutine에서 `p.Send()` 사용 가능
- `streamResponse()` → `provider.Stream(messages, model, onToken, onReasoningToken, reasoningEffort)` 호출. provider가 모든 HTTP/SSE 로직을 캡슐화
- `onToken` 콜백이 `p.Send(StreamChunkMsg(token))` 호출 → Model.Update에서 처리
- `onReasoningToken` 콜백이 `p.Send(StreamReasoningMsg(token))` 호출 → Model.Update에서 동일 패턴으로 처리

### Key Handling
- `handleCommandMode()` — command mode 활성 시 `tea.KeyMsg` 인터셉터
- Command mode에서 `Esc`/`Ctrl+C`는 Quit 대신 palette만 닫기
- `Enter`: 메시지 전송 or command 실행
- `Alt+Enter`: textarea에 newline 삽입 (Bubble Tea v1.3.10에서 `Shift+Enter`는 별도 key type이 없으므로 `Alt+Enter` 사용)
- 그 외 일반 키: textarea로 전달 → `updateCommandMode()` 호출해 `/` prefix 감지
- Viewport KeyMap은 비어 있음 (`viewport.KeyMap{}`) → 키 스크롤 불가, 마우스 휠 또는 slash command로만 스크롤

### Streaming Flow
1. 사용자가 Enter → `messages`에 user + empty assistant 추가 → `streaming = true`
2. Spinner Tick command 시작 → `spinner.TickMsg`가 주기적으로 발생
3. `go m.streamResponse()` → provider.Stream 호출
4. Provider가 토큰마다 `onToken` 콜백 → `p.Send(StreamChunkMsg)`
5. Model.Update의 `StreamChunkMsg` case에서 assistant message content 누적
6. 완료 시 `p.Send(StreamDoneMsg{Err: err})` → `streaming = false`, spinner 정지, textarea 재포커스

### Spinner Lifecycle
- `spinner.Dot` (브라유 점) 사용, color "6" (cyan)
- Enter로 메시지 전송 시 `m.spinner.Tick` command 시작 → 스트리밍 중 계속 ticking
- `spinner.TickMsg`는 Update에서 처리, `streaming == true`일 때만 다음 tick 예약
- `StreamDoneMsg` 수신 → `streaming = false` → 다음 tick 예약 안 함 → spinner 정지
- Spinner는 `statusLine()`에서 `m.spinner.View()`로 렌더링

### SSE Parsing
- `parseSSEStream()` — `data:` 라인에서 JSON content delta 및 reasoning_content delta 추출
- `[DONE]`에서 종료
- Malformed JSON은 skip (에러 없음)
- Scanner buffer: 64KB initial, 1MB max
- 두 콜백 `onToken` (content), `onReasoningToken` (reasoning_content) — 둘 다 nil 허용

### Reasoning Support
- Reasoning models (o1, o3 등)은 SSE delta에 `reasoning_content` 필드를 함께 반환
- `Message.ReasoningContent`는 `json:"-"`로 API 요청에서 제외 (reasoning은 API로 다시 보내지 않음)
- `chatRequest.ReasoningEffort`는 `json:"reasoning_effort,omitempty"` — 빈 값이면 JSON에서 생략
- `refreshViewport()`에서 reasoning content는 이탤릭(ANSI `\033[3m`...`\033[23m`)으로 렌더링, `💭 Reasoning` 헤더 포함
- `StreamReasoningMsg` → Update에서 `StreamChunkMsg`와 동일한 패턴으로 처리 (assistant 슬롯의 `ReasoningContent` 누적)
- lipgloss v1.1.0에 `Italic()` 없으므로 raw ANSI escape 사용

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
- `StreamChunkMsg("token")`, `StreamReasoningMsg("token")`, `StreamDoneMsg{Err: err}`로 스트리밍 시뮬레이션
- `stripANSI()`로 ANSI 이스케이프 제거 후 문자열 검증
- Reasoning rendering 테스트 시 `\033[3m` (italic on), `\033[23m` (italic off) escape 포함 여부 확인
- `NewModel`의 `reasoningEffort` 파라미터로 reasoning depth 설정 테스트
