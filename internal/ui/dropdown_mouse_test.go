package ui

import (
	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strings"
	"testing"
)

func TestBlockTypeAndCachedHashMouseSelection(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	hash := strings.Repeat("a", 64)
	u.store.Put(hash, "hash", "고정 거래", "")
	u.selected = &garden.Command{Path: []string{"query", "block"}, Usage: "demo query block --type=[height|hash] [height|hash] [flags]", Flags: []garden.Flag{{Name: "type", Type: "string", Default: "hash"}}}
	u.commandForm()
	input := u.editor.form.GetFormItem(0).(*tview.InputField)
	drop := u.editor.form.GetFormItem(1).(*tview.DropDown)
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	screen.SetSize(120, 40)
	root := u.screenRoot()
	root.SetRect(0, 0, 120, 40)
	root.Draw(screen)
	focus := func(p tview.Primitive) { u.app.SetFocus(p) }
	// Open the required dropdown, then drag/release on height.
	x, y, _, _ := drop.GetRect()
	drop.MouseHandler()(tview.MouseLeftDown, tcell.NewEventMouse(x, y, tcell.Button1, 0), focus)
	root.Draw(screen)
	optionX := -1
	for row := y + 1; row < y+3; row++ {
		var line strings.Builder
		for col := 0; col < 120; col++ {
			r, _, _, _ := screen.GetContent(col, row)
			line.WriteRune(r)
		}
		if at := strings.Index(line.String(), "height"); at >= 0 {
			optionX = at
			y = row
			break
		}
	}
	if optionX < 0 {
		t.Fatal("dropdown option missing")
	}
	drop.MouseHandler()(tview.MouseLeftUp, tcell.NewEventMouse(optionX, y, tcell.ButtonNone, 0), focus)
	if u.editor.values["flag:type"].Value != "height" {
		t.Fatal("mouse selection failed")
	}
	u.app.SetScreen(screen).SetRoot(root, true).EnableMouse(true)
	u.installInput()
	u.editor.form.SetFocus(0)
	u.app.SetFocus(input)
	sent := false
	u.app.SetAfterDrawFunc(func(tcell.Screen) {
		if sent {
			return
		}
		sent = true
		_, iy, _, _ := input.GetRect()
		// Click the autocomplete candidate below the input, then ensure Ctrl+C works.
		candidateX := -1
		for col := 0; col < 119; col++ {
			r, _, _, _ := screen.GetContent(col, iy+1)
			if r == 'a' {
				candidateX = col
				break
			}
		}
		if candidateX < 0 {
			go u.app.Stop()
			return
		}
		go func() {
			u.app.QueueEvent(tcell.NewEventMouse(candidateX, iy+1, tcell.Button1, 0))
			u.app.QueueEvent(tcell.NewEventMouse(candidateX, iy+1, tcell.ButtonNone, 0))
			u.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
		}()
	})
	if err := u.app.Run(); err != nil {
		t.Fatal(err)
	}
	if input.GetText() != hash {
		t.Fatalf("cached hash mouse selection failed: %q", input.GetText())
	}
}
