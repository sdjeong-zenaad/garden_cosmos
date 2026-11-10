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

## 실행 방식 안내

명령 입력폼 상단에 자동 판단과 그 근거를 표시한다. `기본 실행 (F6)`에서
`자동 판단`, `일반 실행`, `대화형 실행`을 선택할 수 있다. 입력폼에서 F6을
누르면 판단에 맞는 하단 실행 버튼으로 이동한다. F7과 F8은 각각 일반 실행과
대화형 실행으로 유지되며, 자동으로 실행하거나 서로 전환하지 않는다.

- `internal/garden/interaction.go`: 알려진 명령, 도움말의 본문, 활성 플래그를
  이용한 권장 판정. `--yes`는 키링 암호 입력까지 없애지 않는다.
- descriptor 스키마 3의 `interaction`에는 `level`, `reasons`, `source`를 저장한다.
  기존 descriptor는 다시 분석한다. 분석은 버전·도움말만 실행한다.
- `internal/ui/interaction.go`: 실행폼의 플래그 변경에 따른 안내와 사용자 선택 저장.
  `execution-modes.json`은 CLI 경로·home·명령별로 선택을 기억하며 재분석 후에도 유지한다.
  `자동 판단`을 선택하면 해당 명령의 수동 지정을 제거한다.
- `recommended`, `conditional`, `normal`, `unknown`은 권장 등급이며 입력 필요 여부의
  확정값이 아니다. 새 CLI나 플러그인 명령은 미확인으로 남을 수 있다.
