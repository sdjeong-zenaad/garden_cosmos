package ui

import (
	"encoding/json"
	"regexp"
	"strings"

	"garden_cosmos/internal/garden"
	"github.com/rivo/tview"
)

var jsonToken = regexp.MustCompile(`"(?:[^"\\]|\\.)*"\s*:|"(?:[^"\\]|\\.)*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null`)

func colorOutput(raw string) string {
	pretty := garden.Pretty(raw)
	if !json.Valid([]byte(raw)) {
		return tview.Escape(pretty)
	}
	var out strings.Builder
	last := 0
	for _, span := range jsonToken.FindAllStringIndex(pretty, -1) {
		out.WriteString(tview.Escape(pretty[last:span[0]]))
		token := pretty[span[0]:span[1]]
		color := "cyan"
		if strings.HasSuffix(token, ":") {
			color = "lightskyblue"
		} else if strings.HasPrefix(token, `"`) {
			color = "lightgreen"
		} else if token == "true" || token == "false" || token == "null" {
			color = "yellow"
		}
		out.WriteString("[" + color + "]" + tview.Escape(token) + "[-]")
		last = span[1]
	}
	out.WriteString(tview.Escape(pretty[last:]))
	return out.String()
}
