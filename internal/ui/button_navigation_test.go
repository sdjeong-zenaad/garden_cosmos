package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"testing"
)

func TestButtonArrowGroups(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		u := editorFixture(t)
		u.installInput()
		row := tview.NewFlex()
		forward, backward, perpendicular := tcell.KeyRight, tcell.KeyLeft, tcell.KeyDown
		if vertical {
			row.SetDirection(tview.FlexRow)
			forward, backward, perpendicular = tcell.KeyDown, tcell.KeyUp, tcell.KeyRight
		}
		buttons := []*tview.Button{tview.NewButton("one"), tview.NewButton("two"), tview.NewButton("three")}
		for _, b := range buttons {
			row.AddItem(&edgeButton{b}, 0, 1, false)
		}
		u.pages.AddPage("buttons", row, true, true)
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		screen.SetSize(80, 20)
		row.SetRect(0, 0, 60, 9)
		row.Draw(screen)
		u.app.SetFocus(buttons[0])
		press := func(key tcell.Key, want *tview.Button) {
			t.Helper()
			if u.app.GetInputCapture()(tcell.NewEventKey(key, 0, 0)) != nil {
				t.Fatal("arrow not consumed")
			}
			if u.app.GetFocus() != want {
				t.Fatalf("wrong focus for vertical=%v key=%v", vertical, key)
			}
		}
		press(forward, buttons[1])
		press(forward, buttons[2])
		press(forward, buttons[2])
		press(backward, buttons[1])
		press(perpendicular, buttons[1])
		buttons[0].SetDisabled(true)
		press(backward, buttons[1])
		screen.Fini()
	}
}

func TestCommandActionArrowNavigation(t *testing.T) {
	u := editorFixture(t)
	u.installInput()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(140, 40)
	root := u.screenRoot()
	root.SetRect(0, 0, 140, 40)
	root.Draw(screen)
	_, p := u.pages.GetFrontPage()
	page := p.(*commandPage)
	row := page.GetItem(page.GetItemCount() - 1).(*tview.Flex)
	first := row.GetItem(0).(*edgeButton).Button
	second := row.GetItem(2).(*edgeButton).Button
	u.app.SetFocus(first)
	u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if u.app.GetFocus() != second {
		t.Fatal("command action did not move right")
	}
	u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	if u.app.GetFocus() != first {
		t.Fatal("command action did not move left")
	}
}
