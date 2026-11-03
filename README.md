# garden_cosmos · 가든코스모스

## 목적

`gcosm`은 텍스트 터미널에서 Cosmos SDK 계열 CLI를 탐색하고 실행하는 Go로 제작된 TUI앱입니다.
CLI 도움말과 TOML 설정을 분석해 명령 입력 폼을 제공하며, 결과 조회·주소록·클립보드·토큰 단위 계산을 지원합니다.

## 설치

Go 1.23 이상, `make`, 실행할 Cosmos CLI가 필요합니다. Ubuntu 기준이며 권장 터미널 크기는 120×40 이상입니다.

```sh
make install                 # ~/go/bin/gcosm에 설치
make                         # 설치 없이 bin/gcosm 빌드
make install PREFIX=/경로    # /경로/bin/gcosm에 설치
```

`~/go/bin`이 PATH에 없다면 셸 설정에 `export PATH="$HOME/go/bin:$PATH"`를 추가하세요.
로고는 실행 파일에 내장되어 별도 파일이 필요 없습니다.

Go 실행 파일이나 툴체인 버전을 지정할 수 있습니다.

```sh
make toolchain                         # 선택된 Go 버전 확인
make install GO=/usr/local/go/bin/go   # Go 실행 파일 지정
make install GOTOOLCHAIN=go1.26.8      # 버전 지정 예시 (없으면 다운로드)
make install GOTOOLCHAIN=local         # 현재 Go만 사용, 자동 전환 금지
```

### 선택적 도구

`jq`(JSON 쿼리), `bat`(출력 뷰어, cat 대신), `ripgrep`(텍스트 검색, grep 대신), `script`(대화형 실행).
클립보드는 Wayland의 `wl-clipboard`, X11의 `xclip`, KDE의 `qdbus6`/`qdbus` 중 사용 가능한 도구를 이용합니다.

```sh
sudo apt install jq bat ripgrep util-linux
```

## 실행 옵션

```sh
gcosm --cli "$(which cosmd)"   # 첫 실행: CLI 선택 (cosmd 대신 원하는 실행 이름을 넣어서 실행)
gcosm                               # 마지막 CLI 설정으로 실행
gcosm --analyze --refresh           # CLI의 도움말 재분석 후 종료
```

- `--cli PATH`: 사용할 Cosmos CLI 경로.
- `--home PATH`: CLI의 기존 설정 디렉터리.
- `--data-dir PATH`: gcosm 설정·캐시 저장 위치. 기본값은 `${XDG_DATA_HOME:-$HOME/.local/share}/garden_cosmos`.
- `--analyze`: 도움말 분석 후 TUI 없이 종료.
- `--refresh`: 캐시를 무시하고 재분석.
- `--doctor`: 선택 도구 설치 상태 확인.
- `--version`: 앱 버전 출력.
- `--help`: 도움말 출력.

인자 없이 실행했을 때 저장된 CLI 설정이 없으면 도움말을 표시합니다.

## 단축키

화면 하단에 현재 사용할 수 있는 기능을 표시하며, 기능키 영역을 마우스로 클릭해도 실행됩니다.

- **F1**: CLI 확인·변경·재분석.
- **F2**: 최근 주소·해시 및 주소록. 별명으로 고정하거나 입력칸에 삽입.
- **F3 / F4**: 클립보드 복사 / 붙여넣기.
- **F5**: 도움말 재분석.
- **F6**: 설정 조회. 명령 폼에서는 입력칸과 하단 버튼 사이 이동.
- **F7**: 입력 검증 후 일반 실행, 결과를 TUI에 표시.
- **F8**: 대화형 실행. 선택이나 비밀번호 등 실행 중간 키보드 입력이 필요한 명령에 사용.
- **F9**: 결과쿼리 열기/닫기. JSON은 jq, 일반 텍스트는 ripgrep(`rg`)으로 조회.
- **F10**: 종료.
- **F12**: 계산기 열기/닫기. 마지막 입력과 결과를 복원.
- **Tab / Shift+Tab**: 다음 / 이전 항목으로 포커스 이동.
- **Enter**: 선택한 명령·버튼 실행. 명령 검색 중에는 첫 번째 명령의 입력 폼 열기.
- **↑ / ↓**: 목록 이동. 명령 검색 중에는 목록의 마지막 / 현재 항목으로 이동.
- **Esc**: 대화창 닫기 또는 이전 화면으로 이동.
- **Ctrl+C**: 실행 중인 작업 취소, 대기 중에는 종료.
- **마우스 우클릭**: 클립보드 메뉴.

주소록에서는 **Ctrl+N**으로 등록, **Ctrl+E**로 편집, **Del**로 삭제합니다.
**F11**은 터미널의 전체화면 기능을 위해 사용하지 않습니다.
