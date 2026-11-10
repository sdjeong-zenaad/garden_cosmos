package garden

import "strings"

// Interaction is advisory: help does not provide a reliable stdin contract.
// Never execute a command to determine its interaction requirements.
type Interaction struct {
	Level   string   `json:"level"`
	Reasons []string `json:"reasons"`
	Source  string   `json:"source"`
}

// AnalyzeInteraction records the recommendation using declared flag defaults.
func AnalyzeInteraction(c *Command) Interaction { return ResolveInteraction(c, nil) }

// ResolveInteraction considers only enabled flags; an unchecked flag uses the
// CLI's declared default, rather than a remembered but inactive form value.
func ResolveInteraction(c *Command, values map[string]FieldValue) Interaction {
	flag := func(name string) string {
		if v, ok := values["flag:"+name]; ok && v.Enabled {
			return v.Value
		}
		for _, f := range c.Flags {
			if f.Name == name {
				return f.Default
			}
		}
		return ""
	}
	has := func(name string) bool {
		for _, f := range c.Flags {
			if f.Name == name {
				return true
			}
		}
		return false
	}
	result := func(level, source, reason string) Interaction { return Interaction{level, []string{reason}, source} }
	path := strings.Join(c.Path, " ")
	switch path {
	case "tx gov draft-proposal":
		return result("recommended", "known-command", "질문에 응답하며 제안 파일을 작성합니다")
	case "keys import", "keys export", "keys migrate":
		return result("recommended", "known-command", "키 암호 등 터미널 입력이 필요할 수 있습니다")
	case "keys add":
		if flag("recover") == "true" || flag("interactive") == "true" {
			return result("recommended", "flags", "복구 문구 또는 키 생성 정보를 입력합니다")
		}
	}
	// Inspect narrative text only. Common --yes/--keyring help appears even on
	// noninteractive commands and must not become positive evidence by itself.
	narrative := strings.ToLower(strings.SplitN(c.Help, "Usage:", 2)[0] + " " + c.Summary)
	for _, term := range []string{"interactive", "prompt", "password", "confirm", "stdin"} {
		if strings.Contains(narrative, term) {
			return result("conditional", "help", "도움말에 입력 관련 표현이 있습니다: "+term)
		}
	}
	if len(c.Path) > 0 && (c.Path[0] == "query" || c.Path[0] == "q") {
		return result("normal", "known-command", "조회 명령은 보통 터미널 입력 없이 실행됩니다")
	}
	if path == "version" || path == "comet version" || path == "keys list" || path == "keys show" {
		return result("normal", "known-command", "정보 조회 명령은 보통 일반 실행이 가능합니다")
	}
	var reasons []string
	if has("yes") && flag("yes") != "true" && flag("generate-only") != "true" && flag("dry-run") != "true" {
		reasons = append(reasons, "확인 질문이 나올 수 있습니다 (--yes 미설정)")
	}
	keyAccess := len(c.Path) > 0 && (c.Path[0] == "tx" || c.Path[0] == "keys")
	if keyAccess && has("keyring-backend") && flag("dry-run") != "true" {
		backend := flag("keyring-backend")
		if backend != "test" && backend != "memory" {
			reasons = append(reasons, "키링 잠금 해제 또는 암호 입력이 필요할 수 있습니다")
		}
	}
	if keyAccess && flag("ledger") == "true" {
		reasons = append(reasons, "Ledger 장치의 확인이 필요할 수 있습니다")
	}
	if len(reasons) > 0 {
		return Interaction{"conditional", reasons, "flags"}
	}
	if keyAccess && (has("yes") || has("generate-only")) {
		return result("normal", "flags", "현재 옵션에서 알려진 확인·암호 입력 조건이 없습니다 (확정 아님)")
	}
	return result("unknown", "none", "도움말만으로 입력 필요 여부를 판단할 수 없습니다")
}

func (i Interaction) PreferInteractive() bool {
	return i.Level == "recommended" || i.Level == "conditional"
}
func (i Interaction) Label() string {
	switch i.Level {
	case "recommended":
		return "대화형 권장"
	case "conditional":
		return "조건부 대화형 권장"
	case "normal":
		return "일반 실행 가능"
	default:
		return "입력 필요 여부 미확인"
	}
}
