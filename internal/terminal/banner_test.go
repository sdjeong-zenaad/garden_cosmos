package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	for _, color := range []bool{false, true} {
		var b bytes.Buffer
		Banner(&b, "9.8.7", color)
		s := b.String()
		if !strings.Contains(s, "[ Garden Cosmos ]") || !strings.Contains(s, "\n                v9.8.7\n") || !strings.Contains(s, "▀") || !strings.Contains(s, "▄") {
			t.Fatal("missing artwork or version")
		}
		if strings.Contains(s, "\x1b[") != color {
			t.Fatal("color mode ignored")
		}
	}
}
