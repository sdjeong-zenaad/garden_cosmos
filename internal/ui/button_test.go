package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"testing"
)

func TestButtonMarkersAcrossScreens(t *testing.T) {
	u := editorFixture(t)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 35)
	root := u.screenRoot()
	root.SetRect(0, 0, 120, 35)
	check := func(button *tview.Button) {
		t.Helper()
		for _, focused := range []bool{false, true, false} {
			if focused {
				u.app.SetFocus(button)
			} else {
				u.app.SetFocus(u.search)
			}
			root.Draw(s)
			x, y, w, h := button.GetRect()
			y += h / 2
			want := "[  ]"
			if focused {
				want = "[[]]"
			}
			for i, col := range []int{x, x + 1, x + w - 2, x + w - 1} {
				r, _, style, _ := s.GetContent(col, y)
				if r != rune(want[i]) {
					t.Fatalf("%s focused=%v column=%d got %q want %q", button.GetLabel(), focused, col, r, want[i])
				}
				fg, bg, _ := style.Decompose()
				if fg == bg {
					t.Fatal("invisible marker")
				}
			}
		}
	}
	_, page := u.pages.GetFrontPage()
	command := page.(*commandPage)
	check(command.GetItem(1).(*tview.Flex).GetItem(0).(*tview.Flex).GetItem(0).(*tview.Button))
	check(command.GetItem(command.GetItemCount() - 1).(*tview.Flex).GetItem(0).(*edgeButton).Button)
	u.settings()
	_, page = u.pages.GetFrontPage()
	form := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Form)
	check(form.GetButton(0))
	u.alert("message")
	_, page = u.pages.GetFrontPage()
	modal := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*actionModal)
	check(modal.form.GetButton(0))
}

func TestActionModalKeyboardSelection(t *testing.T) {
	app := tview.NewApplication()
	selected := -1
	m := newActionModal().SetText("confirm").AddButtons([]string{"취소", "실행"}).SetDoneFunc(func(i int, _ string) { selected = i })
	app.SetFocus(m)
	focus := func(p tview.Primitive) { app.SetFocus(p) }
	m.InputHandler()(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), focus)
	m.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), focus)
	if selected != 1 {
		t.Fatal("dialog selected wrong action", selected)
	}
}
