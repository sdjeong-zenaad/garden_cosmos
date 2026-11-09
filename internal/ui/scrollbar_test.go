package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strings"
	"testing"
)

func TestMainScrollbars(t *testing.T) {
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
	boxes := []*tview.Box{u.list.Box, u.help.Box, u.output.Box}
	u.list.SetChangedFunc(nil) // Synthetic rows have no descriptor mapping.
	u.list.Clear()
	u.help.SetText("")
	u.output.SetText("")
	root.Draw(screen)
	for _, box := range boxes {
		x, y, w, h := box.GetInnerRect()
		for row := 0; row < h; row++ {
			ch, _, _, _ := screen.GetContent(x+w, y+row)
			if ch != '█' {
				t.Fatalf("empty pane missing scrollbar at row %d: %c", row, ch)
			}
		}
	}
	for i := 0; i < 100; i++ {
		u.list.AddItem(fmt.Sprint(i), "", 0, nil)
	}
	u.help.SetText(strings.Repeat("help line\n", 100))
	u.output.SetText(strings.Repeat("result line\n", 100))
	root.Draw(screen)
	for _, box := range boxes {
		x, y, w, h := box.GetInnerRect()
		first, _, _, _ := screen.GetContent(x+w, y)
		last, _, _, _ := screen.GetContent(x+w, y+h-1)
		if first != '█' || last != '░' {
			t.Fatalf("top position: %c %c", first, last)
		}
		used, _ := root.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x+w, y+h-1, tcell.Button1, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		if !used {
			t.Fatal("scrollbar click ignored")
		}
		root.Draw(screen)
		last, _, _, _ = screen.GetContent(x+w, y+h-1)
		if last != '█' {
			t.Fatal("thumb did not reach bottom")
		}
	}
	if u.list.GetCurrentItem() != 99 || u.editor != nil {
		t.Fatal("command scrollbar executed or selected wrong command")
	}
	// Resize recalculates the track; titles and corners remain untouched.
	screen.SetSize(80, 24)
	root.SetRect(0, 0, 80, 24)
	root.Draw(screen)
	for _, box := range boxes {
		x, y, w, h := box.GetInnerRect()
		ch, _, _, _ := screen.GetContent(x+w, y+h-1)
		if ch != '█' && ch != '░' {
			t.Fatal("resized track missing")
		}
	}
}
