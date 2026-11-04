package ui

import (
	"os"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestPaneFocusAndLocalCopy(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	u.leaveCommand()
	u.output.SetText("result text")
	u.list.AddItem("first", "", 0, nil).AddItem("second", "", 0, nil)
	u.help.SetText("complete help\nsecond line")
	root := u.screenRoot().(*appScreen)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(140, 40)
	root.SetRect(0, 0, 140, 40)
	if root.GetItemCount() != 2 {
		t.Fatal("global clipboard toolbar remains")
	}
	for _, target := range []tview.Primitive{u.help, u.output, u.list} {
		u.app.SetFocus(target)
		root.Draw(screen)
		for _, c := range root.controls {
			if c.target == u.list {
				t.Fatal("command list copy button remains")
			}
		}
		for _, box := range []*tview.Box{u.help.Box, u.output.Box, u.list.Box} {
			want := tview.Styles.PrimitiveBackgroundColor
			if (target == u.help && box == u.help.Box) || (target == u.output && box == u.output.Box) || (target == u.list && box == u.list.Box) {
				want = focusedPaneBackground
			}
			if box.GetBackgroundColor() != want {
				t.Fatal("pane background did not track focus")
			}
			x, y, w, h := box.GetInnerRect()
			// Check actual terminal cells, including text and blank interior.
			if box == u.list.Box {
				y++
			} // preserve the selected item's contrast.
			for _, pos := range [][2]int{{x, y}, {x + w - 1, y + h - 2}} {
				_, _, style, _ := screen.GetContent(pos[0], pos[1])
				_, bg, _ := style.Decompose()
				if bg != want {
					t.Fatalf("interior background: got %v want %v", bg, want)
				}
			}
		}
	}
	u.app.SetFocus(u.help)
	root.Draw(screen)
	x, y, w, _ := u.help.GetRect()
	r, _, _, _ := screen.GetContent(x, y)
	if r != tview.Borders.TopLeftFocus {
		t.Fatal("focused border missing")
	}
	for _, c := range root.controls {
		if c.target != u.help {
			continue
		}
		bx, by, bw, _ := c.button.GetRect()
		if bx+bw != x+w-1 || by != y {
			t.Fatal("copy is not right aligned on title")
		}
		root.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(bx, by, tcell.Button1, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		got, err := os.ReadFile(file)
		if err != nil || string(got) != "complete help\nsecond line" {
			t.Fatalf("copy: %q %v", got, err)
		}
		return
	}
	t.Fatal("pane copy missing")
}

func TestInputControlsAvailabilityAndSpacing(t *testing.T) {
	mockClipboard(t)
	u := editorFixture(t)
	root := u.screenRoot().(*appScreen)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(140, 40)
	root.SetRect(0, 0, 140, 40)
	row := u.editor.form.GetFormItem(1).(*fieldRow)
	field := row.FormItem.(*tview.InputField)
	count := func() int {
		n := 0
		for _, c := range root.controls {
			if c.target == field {
				n++
			}
		}
		return n
	}
	root.Draw(screen)
	if count() != 0 {
		t.Fatal("disabled input has controls")
	}
	row.check.SetChecked(true)
	root.Draw(screen)
	if count() != 2 {
		t.Fatal("enabled input missing controls")
	}
	var copy, paste *tview.Button
	for _, c := range root.controls {
		if c.target == field {
			if c.button.GetLabel() == "복사" {
				copy = c.button
			} else {
				paste = c.button
			}
		}
	}
	x, _, w, _ := field.GetInnerRect()
	cx, _, cw, _ := copy.GetRect()
	px, _, _, _ := paste.GetRect()
	if cx != x+w+1 || px != cx+cw+1 {
		t.Fatal("expected one-cell gaps")
	}
	row.check.SetChecked(false)
	root.Draw(screen)
	if count() != 0 {
		t.Fatal("controls stayed after disabling input")
	}
	t.Setenv("PATH", t.TempDir())
	root.Draw(screen)
	if len(root.controls) != 0 {
		t.Fatal("controls visible without clipboard tools")
	}
}

func TestInputClipboardKeyboardCycle(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	u.installInput()
	root := u.screenRoot().(*appScreen)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(140, 40)
	root.SetRect(0, 0, 140, 40)
	field := u.editor.form.GetFormItem(0).(*tview.InputField)
	field.SetText("42")
	u.app.SetFocus(field)
	root.Draw(screen)
	var copy, paste *tview.Button
	for _, c := range root.controls {
		if c.target == field {
			if c.button.GetLabel() == "복사" {
				copy = c.button
			} else {
				paste = c.button
			}
		}
	}
	press := func(key tcell.Key, want tview.Primitive) {
		t.Helper()
		event := u.app.GetInputCapture()(tcell.NewEventKey(key, 0, 0))
		if event != nil {
			u.pages.InputHandler()(event, func(p tview.Primitive) { u.app.SetFocus(p) })
		}
		root.Draw(screen)
		if u.app.GetFocus() != want {
			t.Fatalf("key %v focus %T want %T", key, u.app.GetFocus(), want)
		}
	}
	press(tcell.KeyTab, copy)
	press(tcell.KeyEnter, copy)
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "42" {
		t.Fatalf("keyboard copy: %q %v", data, err)
	}
	press(tcell.KeyTab, paste)
	field.SetText("")
	press(tcell.KeyEnter, paste)
	if field.GetText() != "42" {
		t.Fatal("keyboard paste failed")
	}
	press(tcell.KeyBacktab, copy)
	press(tcell.KeyBacktab, field)
	press(tcell.KeyTab, copy)
	press(tcell.KeyRight, paste)
	next := u.editor.form.GetFormItem(1).(*fieldRow).check
	press(tcell.KeyTab, next)
	press(tcell.KeyBacktab, paste)
}

func TestDisabledDropdownMatchesFormBackground(t *testing.T) {
	u := editorFixture(t)
	root := u.screenRoot().(*appScreen)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(140, 40)
	root.SetRect(0, 0, 140, 40)
	var drop *tview.DropDown
	for i := 0; i < u.editor.form.GetFormItemCount(); i++ {
		if row, ok := u.editor.form.GetFormItem(i).(*fieldRow); ok {
			if d, ok := row.FormItem.(*tview.DropDown); ok {
				drop = d
				break
			}
		}
	}
	if drop == nil {
		t.Fatal("dropdown fixture missing")
	}
	drop.SetCurrentOption(1)
	u.app.SetFocus(u.editor.form.GetFormItem(0))
	root.Draw(screen)
	x, y, w, _ := drop.GetRect()
	found := false
	for col := x; col < x+w; col++ {
		r, _, style, _ := screen.GetContent(col, y)
		if r == 'j' { // disabled output=json option
			_, bg, _ := style.Decompose()
			if bg != focusedPaneBackground {
				t.Fatalf("disabled option background: %v", bg)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("disabled option not rendered")
	}
}
