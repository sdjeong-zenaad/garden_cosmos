package garden

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Session counts only explicit user CLI executions, never discovery or helper tools.
// It deliberately retains command paths, not arguments, output, or credentials.
type Session struct {
	Total, ExitZero, Failed, Canceled, StartFailures int
	Commands                                         map[string]int
}

func (s *Session) Record(binary string, path []string, started bool, err error, canceled bool) {
	if s == nil {
		return
	}
	if !started {
		s.StartFailures++
		return
	}
	s.Total++
	if canceled {
		s.Canceled++
	} else if err != nil {
		s.Failed++
	} else {
		s.ExitZero++
	}
	if s.Commands == nil {
		s.Commands = map[string]int{}
	}
	key := binary + " " + strings.Join(path, " ")
	s.Commands[strings.TrimSpace(key)]++
}
func (s *Session) Summary(w io.Writer) {
	fmt.Fprintf(w, "\n  실행한 명령: %d종류 · 총 %d회\n  종료 코드 0: %d회 · 오류: %d회 · 취소/시간 초과: %d회\n", len(s.Commands), s.Total, s.ExitZero, s.Failed, s.Canceled)
	if s.StartFailures > 0 {
		fmt.Fprintf(w, "  프로세스 시작 실패: %d회 (실행 횟수 제외)\n", s.StartFailures)
	}
	keys := make([]string, 0, len(s.Commands))
	for k := range s.Commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  - %s: %d회\n", Clean(k), s.Commands[k])
	}
}
