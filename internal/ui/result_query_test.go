package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestJQWindow(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	var rows []string
	for i := 0; i < 150; i++ {
		rows = append(rows, fmt.Sprintf(`{"height":%d}`, i))
	}
	u.raw = "[" + strings.Join(rows, ",") + "]"
	u.render(u.raw, "previous stderr")
	previousText, previousCopy := u.output.GetText(false), u.displayOutput
	u.openResultQuery()
	_, p := u.pages.GetFrontPage()
	v := queryResultPane(p)
	if v == nil || !strings.Contains(v.GetText(true), `149`) {
		t.Fatal("full JSON missing")
	}
	page := p.(*tview.Flex)
	source := page.GetItem(0).(*queryView)
	q := page.GetItem(1).(*tview.InputField)
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	screen.SetSize(80, 24)
	page.SetRect(0, 0, 80, 23)
	page.Draw(screen)
	x, y, w, _ := v.GetInnerRect()
	ch, _, _, _ := screen.GetContent(x+w, y)
	if ch != '█' {
		t.Fatalf("scrollbar missing: %q", ch)
	}
	qx, qy, _, _ := q.GetRect()
	if qx < 0 || (qy < 10 || qy > 11) {
		t.Fatalf("query not centered: %d", qy)
	}
	page.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if !v.HasFocus() {
		t.Fatal("Tab did not focus JSON")
	}
	v.InputHandler()(tcell.NewEventKey(tcell.KeyEnd, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	page.Draw(screen)
	row, _ := v.GetScrollOffset()
	if row == 0 {
		t.Fatal("cannot scroll full result")
	}
	page.GetInputCapture()(tcell.NewEventKey(tcell.KeyBacktab, 0, 0))
	if !q.HasFocus() {
		t.Fatal("reverse Tab")
	}
	page.GetInputCapture()(tcell.NewEventKey(tcell.KeyBacktab, 0, 0))
	if !source.HasFocus() || queryResultPane(page) != source {
		t.Fatal("reverse Tab did not focus source")
	}
	page.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	_, _, _, sh := source.GetRect()
	_, _, _, rh := v.GetRect()
	if sh-rh > 1 || rh-sh > 1 {
		t.Fatal("panes are not equal halves")
	}
	q.SetText(".[149].height")
	u.app.SetScreen(screen).SetRoot(u.screenRoot(), true)
	u.installInput()
	phase := 0
	u.app.SetAfterDrawFunc(func(tcell.Screen) {
		if phase == 0 {
			phase = 1
			go u.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		} else if !u.busy && strings.TrimSpace(v.GetText(true)) == "149" {
			phase = 2
			go u.app.Stop()
		}
	})
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		u.app.Stop()
		t.Fatal("query did not update open window")
	}
	if phase != 2 || u.raw != "["+strings.Join(rows, ",")+"]" {
		t.Fatal("query changed source")
	}
	if _, front := u.pages.GetFrontPage(); front != page {
		t.Fatal("query closed window")
	}
	if !strings.Contains(source.GetText(true), `148`) {
		t.Fatal("source pane replaced by query")
	}
	if u.output.GetText(false) != previousText || u.displayOutput != previousCopy {
		t.Fatal("query modified main result")
	}
	u.close()
	if u.output.GetText(false) != previousText || u.displayOutput != previousCopy {
		t.Fatal("closing changed main result")
	}

}

func TestTextQueryExecutionAndRestore(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	u.raw = "INFO ok\nERROR failed\n"
	u.rawErr = "WARN retry\n"
	u.render(u.raw, u.rawErr)
	previous, copyText := u.output.GetText(false), u.displayOutput
	u.resultQuery()
	_, page := u.pages.GetFrontPage()
	result := queryResultPane(page)
	if result == nil {
		t.Fatal("text query did not open")
	}
	q := page.(*tview.Flex).GetItem(1).(*tview.InputField)
	if q.GetLabel() != "rg > " {
		t.Fatal("wrong query mode")
	}
	q.SetText("ERROR|WARN")
	screen := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(screen).SetRoot(u.screenRoot(), true)
	u.installInput()
	phase := 0
	u.app.SetAfterDrawFunc(func(tcell.Screen) {
		if phase == 0 {
			phase = 1
			go u.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		} else if phase == 1 && !u.busy && result.GetText(true) == "ERROR failed\nWARN retry\n" {
			phase = 2
			go u.app.QueueEvent(tcell.NewEventKey(tcell.KeyF9, 0, 0))
		} else if phase == 2 {
			if name, _ := u.pages.GetFrontPage(); name == "main" {
				phase = 3
				go u.app.Stop()
			}
		}
	})
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		u.app.Stop()
		t.Fatal("text query event loop stalled")
	}
	if phase != 3 || u.output.GetText(false) != previous || u.displayOutput != copyText {
		t.Fatal("text query changed original output")
	}
}

func TestTextQueryMissingRipgrep(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	u := editorFixture(t)
	u.raw = "plain text"
	u.resultQuery()
	_, page := u.pages.GetFrontPage()
	modal := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*actionModal)
	if text := modal.text.GetText(true); !strings.Contains(text, "rg가 없습니다") || !strings.Contains(text, "sudo apt install ripgrep") {
		t.Fatal(text)
	}
}
