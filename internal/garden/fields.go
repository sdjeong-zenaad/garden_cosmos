package garden

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Field struct {
	Key, Name, Type, Default, Description string
	Flag, Required, Variadic              bool
	Choices                               []string
}
type FieldValue struct {
	Enabled bool
	Value   string
}
type Preferences map[string]map[string]FieldValue

var placeholders = regexp.MustCompile(`(?:<([^>]+)>|\[([^\[\]]+)\])(?:\.\.\.)?`)
var alternatives = regexp.MustCompile(`\(([a-zA-Z0-9_-]+(?:\|[a-zA-Z0-9_-]+)+)\)`)

// Parse flag-owned placeholders before collecting positional placeholders.
var usageFlagValue = regexp.MustCompile(`(\[)?--([a-zA-Z0-9][a-zA-Z0-9_-]*)(?:=|[ \t]+)(\[[^\[\]]+\]|<[^>]+>)`)
var usageChoices = regexp.MustCompile(`^[a-zA-Z0-9_-]+(?:\|[a-zA-Z0-9_-]+)+$`)

func Fields(c *Command) []Field {
	type usageFlag struct {
		required bool
		choices  []string
	}
	explicit := map[string]usageFlag{}
	usage := usageFlagValue.ReplaceAllStringFunc(c.Usage, func(token string) string {
		m := usageFlagValue.FindStringSubmatch(token)
		value := m[3][1 : len(m[3])-1]
		info := usageFlag{required: m[1] == ""}
		if usageChoices.MatchString(value) {
			info.choices = strings.Split(value, "|")
		}
		explicit[m[2]] = info
		return ""
	})

	var fields []Field
	for _, m := range placeholders.FindAllStringSubmatch(usage, -1) {
		name := m[1]
		required := true
		if name == "" {
			name = m[2]
		}
		if name == "flags" || name == "command" || strings.HasPrefix(name, "--") {
			continue
		}
		variadic := strings.HasSuffix(name, "...") || strings.HasSuffix(m[0], "...")
		name = strings.TrimSuffix(name, "...")
		if name == "" {
			continue
		}
		fields = append(fields, Field{Key: fmt.Sprintf("arg:%d", len(fields)), Name: name, Type: argumentType(name), Required: required, Variadic: variadic, Description: "위치 인자 · " + name + " · 필수"})
	}
	flagStart := len(fields)
	for _, f := range c.Flags {
		if f.Name == "help" {
			continue
		}
		required := strings.Contains(strings.ToLower(f.Description), "(required)") || strings.Contains(strings.ToLower(f.Description), "[required]")
		field := Field{Key: "flag:" + f.Name, Name: f.Name, Type: f.Type, Default: f.Default, Description: f.Description, Flag: true, Required: required}
		if m := alternatives.FindStringSubmatch(f.Description); m != nil {
			field.Choices = strings.Split(m[1], "|")
		}
		if info, ok := explicit[f.Name]; ok {
			field.Required = field.Required || info.required
			if len(info.choices) > 0 {
				field.Choices = info.choices
			}
		}
		if f.Type == "bool" {
			field.Choices = []string{"true", "false"}
			if field.Default == "" {
				field.Default = "false"
			}
		}
		fields = append(fields, field)
	}
	flags := fields[flagStart:]
	sort.SliceStable(flags, func(i, j int) bool {
		return flags[i].Required && !flags[j].Required
	})
	return fields
}
func argumentType(name string) string {
	n := strings.ToLower(name)
	switch {
	case n == "height" || n == "proposal-id" || n == "proposal_id" || n == "code-id" || n == "code_id":
		return "uint64"
	case n == "address" || n == "recipient" || n == "contract-address":
		return "address"
	case n == "coins":
		return "coins"
	default:
		return "string"
	}
}

// Remember flags across commands, except values containing credentials.
func CommonFlag(name string) bool {
	return !secretKey.MatchString(name) && !strings.Contains(strings.ToLower(name), "seed")
}

var addressPattern = regexp.MustCompile(`^(?:0x[0-9a-fA-F]{40}|[a-z][a-z0-9]{1,82}1[023456789acdefghjklmnpqrstuvwxyz]{6,100})$`)
var coinPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?[a-zA-Z][a-zA-Z0-9/:._-]{2,127}(?:,[0-9]+(?:\.[0-9]+)?[a-zA-Z][a-zA-Z0-9/:._-]{2,127})*$`)

func ValidateField(f Field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("값을 입력하세요")
	}
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("NUL 문자는 사용할 수 없습니다")
	}
	if len(f.Choices) > 0 {
		for _, c := range f.Choices {
			if value == c {
				return nil
			}
		}
		return fmt.Errorf("다음 중 선택하세요: %s", strings.Join(f.Choices, ", "))
	}
	kind := f.Type
	if f.Flag {
		switch f.Name {
		case "gas":
			if value == "auto" {
				return nil
			}
			kind = "uint64"
		case "fees", "gas-prices":
			kind = "coins"
		case "gas-adjustment":
			kind = "float64"
		}
	}
	switch kind {
	case "bool":
		if value != "true" && value != "false" {
			return fmt.Errorf("true 또는 false를 선택하세요")
		}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		bits := 64
		digits := strings.TrimPrefix(strings.TrimPrefix(kind, "uint"), "int")
		if digits != "" {
			bits, _ = strconv.Atoi(digits)
		}
		var e error
		if strings.HasPrefix(kind, "uint") {
			_, e = strconv.ParseUint(value, 10, bits)
		} else {
			_, e = strconv.ParseInt(value, 10, bits)
		}
		if e != nil {
			return fmt.Errorf("%s 범위의 정수여야 합니다", kind)
		}
	case "float32", "float64":
		v, e := strconv.ParseFloat(value, 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("유한한 숫자를 입력하세요")
		}
	case "duration":
		if _, e := time.ParseDuration(value); e != nil {
			return fmt.Errorf("시간 단위를 포함하세요 (예: 30s, 2m)")
		}
	case "address":
		if !addressPattern.MatchString(value) {
			return fmt.Errorf("Bech32 또는 0x 주소 형식이어야 합니다")
		}
	case "coins":
		if !coinPattern.MatchString(value) {
			return fmt.Errorf("금액과 denom을 입력하세요 (예: 100uatom, 0.025axpla)")
		}
	}
	if f.Flag && f.Name == "node" {
		u, e := url.Parse(value)
		if e != nil || u.Host == "" || (u.Scheme != "tcp" && u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("tcp/http/https://host:port 형식이어야 합니다")
		}
	}
	return nil
}

// Validate enabled form values before building argv without shell interpolation.
func FormArgs(c *Command, fields []Field, values map[string]FieldValue) ([]string, error) {
	gap := false
	for _, f := range fields {
		v := values[f.Key]
		if !v.Enabled && !f.Required {
			if !f.Flag {
				gap = true
			}
			continue
		}
		if !f.Flag && gap {
			return nil, fmt.Errorf("%s: 앞의 위치 인자를 먼저 활성화하세요", f.Name)
		}
		vs := []string{v.Value}
		if f.Variadic {
			vs = strings.Fields(v.Value)
			if len(vs) == 0 {
				return nil, fmt.Errorf("%s: 값을 입력하세요", f.Name)
			}
		}
		for _, value := range vs {
			if e := ValidateField(f, value); e != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, e)
			}
		}
	}
	return CommandArgs(c, fields, values), nil
}

// CommandArgs also builds incomplete previews, keeping the order from Usage.
func CommandArgs(c *Command, fields []Field, values map[string]FieldValue) []string {
	positions := map[string]int{}
	usage := []byte(c.Usage)
	for _, loc := range usageFlagValue.FindAllStringSubmatchIndex(c.Usage, -1) {
		positions["flag:"+c.Usage[loc[4]:loc[5]]] = loc[0]
		for i := loc[0]; i < loc[1]; i++ {
			usage[i] = ' '
		}
	}
	flagPosition := len(usage)
	argIndex := 0
	for _, loc := range placeholders.FindAllStringSubmatchIndex(string(usage), -1) {
		end := loc[1]
		if strings.HasSuffix(string(usage[loc[0]:end]), "...") {
			end -= 3
		}
		name := string(usage[loc[0]+1 : end-1])
		if name == "flags" {
			flagPosition = loc[0]
			continue
		}
		if name == "command" || strings.HasPrefix(name, "--") || name == "" {
			continue
		}
		positions[fmt.Sprintf("arg:%d", argIndex)] = loc[0]
		argIndex++
	}
	ordered := append([]Field(nil), fields...)
	position := func(f Field) int {
		if p, ok := positions[f.Key]; ok {
			return p
		}
		return flagPosition
	}
	sort.SliceStable(ordered, func(i, j int) bool { return position(ordered[i]) < position(ordered[j]) })
	args := append([]string{}, c.Path...)
	var flags, positional []string
	needsSeparator := false
	for _, f := range ordered {
		v := values[f.Key]
		if !v.Enabled && !f.Required {
			continue
		}
		if f.Flag {
			value := "--" + f.Name + "=" + v.Value
			args = append(args, value)
			flags = append(flags, value)
			continue
		}
		vs := []string{v.Value}
		if f.Variadic {
			vs = strings.Fields(v.Value)
		}
		for _, value := range vs {
			if strings.HasPrefix(value, "-") {
				needsSeparator = true
			}
		}
		args = append(args, vs...)
		positional = append(positional, vs...)
	}
	// Quoting does not stop pflag interpreting a dash-prefixed value as a
	// flag. Only these exceptional values require flags before the separator.
	if needsSeparator {
		args = append(append([]string{}, c.Path...), flags...)
		args = append(args, "--")
		args = append(args, positional...)
	}
	return args
}
