# 개발 안내

## 코드 위치

- `cmd/gcosm/main.go`: 실행 옵션, 단일 인스턴스 잠금, 시작·종료 처리.
- `internal/garden/model.go`, `storage.go`: 데이터 구조와 JSON5 읽기·원자적 JSON 저장.
- `internal/garden/analyze.go`: CLI 식별, 도움말 탐색, descriptor 캐시와 TOML 읽기.
- `internal/garden/fields.go`: usage 분석, 입력 검증, 셸을 거치지 않는 argv 생성.
- `internal/garden/process.go`, `interactive.go`, `query.go`: 일반 실행, PTY 실행, jq/rg 조회.
- `internal/garden/capture.go`, `recent.go`: 공개 주소·해시 수집과 최근 목록·주소록 관리.
- `internal/garden/calculator.go`: 부동소수점 오차 없는 유리수 계산과 단위 변환.
- `internal/ui/app.go`, `input.go`: 앱 수명 주기, 메인 화면, 전역 키 입력.
- `internal/ui/profile.go`, `browser.go`: CLI 설정·분석, 명령 검색·선택.
- `internal/ui/command.go`, `execution.go`: 명령 입력폼, 확인과 실행 결과 처리.
- `internal/ui/focus.go`, `local_controls.go`, `screen.go`: 포커스 순서, 인라인 버튼, 화면 렌더링.
- `internal/ui/calculator.go`, `deferred_save.go`: 비동기 계산과 지연 저장.
- `internal/ui/result_query.go`, `recent.go`, `clipboard.go`: 결과 조회, 주소 선택, 클립보드 동작.
- `internal/terminal`: 바이너리에 내장된 시작 배너.

## 변경 시 지킬 동작

- CLI 분석은 버전과 도움말만 실행한다. 분석 과정에서 init·트랜잭션을 실행하지 않는다.
- descriptor 재사용은 스키마, 실행 경로, 해시, 버전, 요청 home이 모두 일치할 때만 허용한다.
- `visibleCommands`는 화면에 표시한 목록의 행과 동일한 순서다. 목록 갱신 시 함께 교체하고 선택 시 다시 검색하지 않는다.
- UI 상태 변경은 tview 이벤트 루프에서 처리한다. Draw 중에는 SetFocus를 호출하지 않고 이벤트 루프에 예약한다.
- Tab 순서는 `focus.go`에서 관리한다. 인라인 버튼은 입력칸과 실제 키보드 포커스를 공유하지 않는다.
- 계산은 취소 가능한 백그라운드 작업이다. 결과 적용 전 입력 세대와 현재 페이지를 확인한다.
- 지연 저장에는 변경되지 않는 상태 복사본을 넘기고, 계산기를 닫거나 앱을 종료할 때 Flush한다.
- 금액은 `big.Rat`을 사용한다. 최소 단위는 정수만 삽입하고, denom의 `/`는 분수 기호로 해석하지 않는다.
- 일반 실행은 stdin을 제공하지 않는다. 입력이 필요한 명령은 사용자가 대화형 실행을 선택한다.
- 키 관리 출력은 기본적으로 수집하지 않는다. `keys list`는 구조화된 공개 주소 필드만 수집한다.
- 저장 파일은 JSON5로 읽지만, CLI 출력·jq 등 표준 JSON 경계의 파싱 규칙은 유지한다.

## 검증

```bash
make test TESTFLAGS='-race -count=1 -timeout 30s'
make vet
make build
go test ./internal/garden -run '^$' -bench 'Benchmark(FormatRat|Flatten)$' -benchmem
```

테스트는 임시 데이터 폴더와 모의 CLI·터미널을 사용한다. 실제 체인 전송이나 데스크톱/SSH 터미널의 시각적 확인을 대신하지 않는다. 성능 수치는 같은 Go 버전과 장비에서 비교한다.
