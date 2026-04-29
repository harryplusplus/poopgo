# AGENTS

## Hindsight Bank ID
poopgo

## Project
PoopGo — Bubble Tea v2 기반 터미널 AI 채팅 클라이언트.
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
| `cmd/poopgo/main.go` | 진입점 — provider 선택, Bubble Tea Program 생성 |
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
| `POOPGO_REASONING_EFFORT` | *(empty → disabled)* | Reasoning depth: `"low"`, `"medium"`, `"high"`, `"xhigh"`, `"max"` (reasoning models only) |
| `POOPGO_TEMPERATURE` | *(empty → API default)* | Sampling temperature `0.0`–`2.0` (e.g., `"0.7"`) |

## Key Patterns

### Provider Architecture
- `StreamProvider` interface: `Stream(messages []Message, model string, onToken, onReasoningToken func(string), reasoningEffort, temperature string) error`
- `RealProvider`: OpenAI 호환 `/chat/completions` API에 HTTP POST → SSE 파싱. `temperature`가 비어있지 않으면 `strconv.ParseFloat`로 파싱해 `chatRequest.Temperature`에 `*float32`로 주입.
- `FakeProvider`: 마지막 user message를 echo + banner. `reasoningEffort`가 설정되면 가짜 reasoning 토큰을 먼저 emit. `temperature`가 설정되면 echo에 🌡️ 표시. API 호출 없음, 테스트용
- `main.go`에서 `POOPGO_PROVIDER` env var에 따라 provider 선택
- `reasoningEffort`와 `temperature`는 `main.go` → `NewModel` → `streamResponse()` → `provider.Stream()`으로 전달
- Model은 provider만 바라보고, HTTP/SSE 디테일을 모름 (의존성 역전)

### Model Lifecycle
- `main.go`에서 `POOPGO_API_KEY` 미설정 + provider가 fake가 아니면 즉시 stderr 출력 후 `os.Exit(1)` (fail-fast). 더 이상 `initErr`를 Model에 전달하지 않음.
- `NewModel()` 생성 후 `SetProgram(p *tea.Program)` 호출해야 goroutine에서 `p.Send()` 사용 가능
- `streamResponse()` → `provider.Stream(messages, model, onToken, onReasoningToken, reasoningEffort, temperature)` 호출. provider가 모든 HTTP/SSE 로직을 캡슐화
- `onToken` 콜백이 `p.Send(StreamChunkMsg(token))` 호출 → Model.Update에서 처리
- `onReasoningToken` 콜백이 `p.Send(StreamReasoningMsg(token))` 호출 → Model.Update에서 동일 패턴으로 처리

### Key Handling
- `handleCommandMode()` — command mode 활성 시 `tea.KeyPressMsg` 인터셉터
- Command mode에서 `Esc`/`Ctrl+C`는 palette만 닫기 (Quit 안 함)
- Normal mode에서 `Esc`는 no-op, `Ctrl+C`만 Quit
- `Enter`: 메시지 전송 or command 실행
- `Shift+Enter`: textarea에 newline 삽입 (Bubble Tea v2의 Kitty Keyboard Protocol 네이티브 지원 — `tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}`)
- `↑`/`↓`: normal mode → viewport 1줄 스크롤 (ScrollUp/ScrollDown). command mode → palette 선택 이동. textarea로 전달 안 함.
- 그 외 일반 키: textarea로 전달 → `updateCommandMode()` 호출해 `/` prefix 감지
- Viewport KeyMap은 비어 있음 (`viewport.KeyMap{}`)
- 마우스 이벤트: `MouseModeNone` → 터미널이 네이티브 처리 (드래그 선택, 휠 스크롤)

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
- `refreshViewport()`에서 reasoning content는 `ansi.Style{}.Italic(true).Styled()`로 이탤릭 렌더링, `💭 Reasoning` 헤더는 `sysStyle.Render()`로 노란색. `ansi.Style.Styled()`는 block 패딩이 없어 `\n\n` 개행 구조를 보존함 (#18).
- `StreamReasoningMsg` → Update에서 `StreamChunkMsg`와 동일한 패턴으로 처리 (assistant 슬롯의 `ReasoningContent` 누적)
- **중요**: reasoning content에 `sysStyle.Render()`를 적용하지 말 것. lipgloss v2 `Render()`는 멀티라인 각 줄을 최대 너비로 공백 패딩하여 빈 줄이 사라지고 `\n\n` 패턴이 깨짐 (#18).
- **중요**: reasoning content는 `collapseNewlines()`로 연속 `\n`을 최대 2개로 축소 후 렌더링 (방어적).

### View (v2 Declarative)
- `View()`는 `tea.View`를 반환 — `tea.NewView(content)`로 생성
- `v.AltScreen = true`
- `v.MouseMode = tea.MouseModeNone` — 마우스 이벤트 off, 터미널이 네이티브 텍스트 선택 및 스크롤 처리
- bubbles 컴포넌트(textarea, viewport, spinner)의 `View()`는 여전히 `string` 반환

### Command Palette
- `/` 입력 → slash command palette 활성화
- Commands: `/help`, `/scroll-top`, `/scroll-bottom`
- `/scroll-up`, `/scroll-down` 제거 — 터미널(Ghostty)이 네이티브 스크롤 처리
- `updateCommandMode()`가 textarea 값 기반으로 `filteredCommands` 필터링
- `executeCommand()`가 실제 동작 수행 (viewport `GotoTop()`/`GotoBottom()` 호출)

### Layout (`applyLayout()`)
- `applyLayout()` — viewport height를 현재 terminal 크기와 command palette 상태에 따라 재계산
- Normal mode: viewport height = terminal height − 5 (separator 1 + textarea 3 + status 1)
- Command mode: viewport height = terminal height − 5 − (len(commands) + 2) — palette가 viewport 아래 공간을 차지하므로 viewport를 축소하여 clipping 방지 (#33)
- commands 개수에 따라 overhead 동적 계산 (현재 3개: /help, /scroll-top, /scroll-bottom)
- 호출 시점: `WindowSizeMsg` 수신, `updateCommandMode()` (palette 진입), `exitCommandMode()` (palette 종료)
- 최소 viewport height는 1로 clamp

### Command Palette Rendering
- `renderCommandPalette()` — dark background panel (`Background("236")`) + `Width(m.width)` → solid block visually separate from viewport
- Header: bold accent color (12), footer: dim (8)
- Selected item: bright black bg (100) + white fg (97); desc: light gray (7) on dark bg for readability
- Pattern: build `[]string` lines, join with `"\n"`, wrap in `panelStyle.Render()` — lipgloss inner bg overrides outer per cell, ANSI reset resumes outer
- Regression test: verify exact ANSI sequences (`48;5;236` for panel bg, `97;100m` for selected) — lipgloss renders 0-15 colors as basic SGR codes, not 256-color format

## Library Usage Guidelines

**모르는 라이브러리는 반드시 소스코드와 문서를 확인할 것.** 함수 몇 개만으로 대충 구현하지 말고, 라이브러리 설계 의도에 맞게 쓸 것.

- `charm.land/lipgloss/v2` — [README](https://github.com/charmbracelet/lipgloss), [pkg.go.dev](https://pkg.go.dev/charm.land/lipgloss/v2), `style.go` 소스
- `charm.land/bubbles/v2` — [pkg.go.dev](https://pkg.go.dev/charm.land/bubbles/v2), 각 컴포넌트 소스
- `charm.land/bubbletea/v2` — [pkg.go.dev](https://pkg.go.dev/charm.land/bubbletea/v2), `tea.go` 소스
- `github.com/charmbracelet/x/ansi` — [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/x/ansi), 저수준 ANSI 스타일링

### lipgloss Style.Render() vs ansi.Style.Styled()

| | `lipgloss.Style.Render()` | `ansi.Style.Styled()` |
|---|---|---|
| 용도 | Block 레이아웃 (padding, margin, border, align) | Inline 텍스트 스타일링 |
| 멀티라인 | 모든 줄을 최대 너비로 공백 패딩 → `\n\n` 파괴 | `\n` 보존, 순수 ANSI escape만 적용 |
| 사용처 | 헤더, 패널, 경계선 있는 UI 요소 | 채팅 메시지 본문, 개행 구조 보존이 중요한 콘텐츠 |

**결론**: 개행 구조를 보존해야 하는 멀티라인 콘텐츠에는 `ansi.Style.Styled()`를 사용할 것.

## Dependencies
| Module | Version | Usage |
|---|---|---|
| `charm.land/bubbletea/v2` | v2.0.6 | Elm 아키텍처 TUI 프레임워크 (v2 — Kitty Keyboard Protocol 네이티브 지원, declarative View) |
| `charm.land/bubbles/v2` | v2.1.0 | textarea, viewport, spinner 컴포넌트 |
| `charm.land/lipgloss/v2` | v2.0.3 | 스타일링 (block layout) |
| `github.com/charmbracelet/x/ansi` | (lipgloss 의존성) | 저수준 ANSI 스타일링 (inline — 개행 보존) |

## Known Constraints
- **Shift+Enter 네이티브 지원**: Bubble Tea v2로 마이그레이션하여 Kitty Keyboard Protocol 네이티브 지원. `Alt+Enter`는 제거됨. `tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}`로 감지.
- **마우스 드래그 텍스트 선택**: `MouseModeNone`으로 설정하여 터미널(Ghostty 등)이 네이티브 텍스트 선택을 처리. 선택 후 Cmd+C로 클립보드 복사. 마우스 휠/터치패드 스크롤도 터미널이 처리.
- **키보드 스크롤**: ↑/↓ 키로 1줄씩 스크롤 (viewport.ScrollUp/ScrollDown). command mode에서는 palette 내비게이션으로 전환.
- **Viewport KeyMap은 비어 있음** (`viewport.KeyMap{}`) → viewport 자체 키바인딩 없음. 스크롤은 Model.Update에서 ↑/↓를 직접 인터셉트.
- **Viewport SoftWrap 활성화**: `SoftWrap = true`로 설정되어 viewport 폭을 넘는 긴 줄은 자동으로 줄바꿈됨. `bubbles/v2` viewport는 기본값 `SoftWrap = false`이며, false일 경우 `ansi.Cut()`으로 초과분을 잘라내 가로스크롤을 의도하지만 KeyMap이 비어 있어 접근 불가능하므로 반드시 true로 설정해야 함. SoftWrap은 `ansi.Cut(line, idx, maxWidth+idx)`로 문자 단위 분할하며, ANSI escape sequence를 올바르게 처리함.
- **lipgloss v2 `Style.Render()` 멀티라인 패딩**: lipgloss v2의 `Render()`는 멀티라인 문자열의 각 줄을 최대 너비로 공백 패딩한다. `\n\n` 패턴이 `\n<spaces>\n`으로 변형되어 빈 줄 감지가 깨지므로, 개행 구조가 중요한 콘텐츠에는 `ansi.Style.Styled()`를 대신 사용할 것 (#18). `ansi.Style`은 순수 인라인 스타일링만 적용한다.
- **Reasoning content 연속 개행 정규화**: `collapseNewlines()`로 렌더링 전에 연속 `\n`을 최대 2개로 축소 (방어적).
- **TUI 테스트 한계**: `tea.Program.Run()`은 실제 터미널 필요. Model.Update에 KeyPressMsg 직접 주입하는 방식으로 키 입력 테스트.

## Testing Guidelines
- 모든 테스트는 네트워크 불필요 (FakeProvider 사용)
- `newTestModel()` 헬퍼로 FakeProvider 기반 Model 생성 (viewport.SetWidth/SetHeight 호출 필요 — v2 viewport.New()는 초기 크기 미설정)
- `tea.KeyPressMsg{Text: "h"}`로 문자 입력 시뮬레이션 (v2는 `Text string` 필드 사용)
- `tea.KeyPressMsg{Code: tea.KeyEnter}`로 Enter 시뮬레이션
- `tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}`로 Shift+Enter 시뮬레이션 (newline 삽입)
- `tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}`로 Ctrl+C 시뮬레이션
- `tea.KeyPressMsg{Code: tea.KeyEsc}`로 Esc 시뮬레이션
- `tea.KeyPressMsg{Code: tea.KeyBackspace}`로 Backspace 시뮬레이션
- `tea.KeyPressMsg{Code: tea.KeyUp}` / `tea.KeyPressMsg{Code: tea.KeyDown}`으로 키보드 스크롤 시뮬레이션
- `tea.KeyPressMsg{Code: tea.KeyPgUp}` / `tea.KeyPressMsg{Code: tea.KeyPgDown}`은 사용 안 함 (제거됨)
- `StreamChunkMsg("token")`, `StreamReasoningMsg("token")`, `StreamDoneMsg{Err: err}`로 스트리밍 시뮬레이션
- `stripANSI()`로 ANSI 이스케이프 제거 후 문자열 검증 — `GetContent()`에서 ANSI 코드가 개행 사이에 끼어 `\n\n` 패턴을 직접 찾을 수 없으므로 반드시 stripANSI 선적용
- `m.View().Content`로 View 문자열 검증 (v2 View()는 `tea.View` 반환)
- Reasoning rendering 테스트 시 `\033[3m` (italic on), `\033[m` (SGR reset) escape 포함 여부 확인 (`ansi.Style{}.Italic(true)` 사용)
- `NewModel`의 `reasoningEffort`, `temperature` 파라미터로 설정 테스트
- Viewport SoftWrap 테스트: `m.viewport.SoftWrap`이 true인지 확인. `SetWidth(20)` 같은 좁은 폭에서 긴 문자열로 `refreshViewport()` 후 `m.viewport.TotalLineCount() > 1` 확인 (줄바꿈 발생). `m.viewport.GetContent()`로 전체 콘텐츠 보존 여부 확인.
- `newTestModel()` 헬퍼는 FakeProvider 사용, `NewModel("sk-test", "https://api.openai.com/v1", "gpt-4o", "", "", NewFakeProvider())` — `reasoningEffort`, `temperature`는 빈 문자열 (6개 인자)
- Layout 테스트: `m.viewport.Height()`로 viewport 높이 검증. command mode 진입 시 `updateCommandMode()` 호출 후 `m.viewport.Height()`가 축소되었는지 확인. `exitCommandMode()` 후 원복 확인. `WindowSizeMsg`로 resize 시 command mode 상태 반영 확인.
- **스트리밍 통합 테스트**: `endToEndStreamReasoning()` 헬퍼로 provider.Stream()을 동기 호출하고 토큰을 Update()에 직접 주입하여 전체 스트리밍 플로우 검증. goroutine 불필요.
