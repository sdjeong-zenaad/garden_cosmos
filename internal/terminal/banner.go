package terminal

import (
	_ "embed"
	"fmt"
	"io"
	"strings"

	"garden_cosmos/internal/garden"
)

// icon is the literal ANSI artwork from docs/icon_px.sh; no shell runs at startup.
//
//go:embed icon.ansi
var icon string

func Banner(w io.Writer, version string, color bool) {
	art := icon
	if !color {
		art = garden.Clean(art)
	}
	fmt.Fprint(w, art)
	// The flower occupies columns 2..35 (zero-based), centered at 18.5.
	label := "v" + version
	padding := max(0, (38-len(label))/2)
	fmt.Fprintf(w, "\n%s%s\n\n", strings.Repeat(" ", padding), label)
}
