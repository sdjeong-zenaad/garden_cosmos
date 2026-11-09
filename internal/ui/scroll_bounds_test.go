package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strings"
	"testing"
)

func TestPageUpAtTopKeepsTextVisible(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	root := u.screenRoot().(*appScreen)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)
	root.SetRect(0, 0, 120, 40)
	u.render(strings.Repeat("visible-result-line\n", 80), "")
	u.app.SetFocus(u.output)
	root.Draw(screen)
	for i := 0; i < 3; i++ {
		u.output.InputHandler()(tcell.NewEventKey(tcell.KeyPgUp, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		root.Draw(screen)
		x, y, _, _ := u.output.GetInnerRect()
		ch, _, _, _ := screen.GetContent(x, y+1)
		if ch != 'v' {
			t.Fatalf("PgUp %d hid content: %q", i+1, ch)
		}
		row, _ := u.output.GetScrollOffset()
		if row != 0 {
			t.Fatal("offset at top", row)
		}
	}
}
