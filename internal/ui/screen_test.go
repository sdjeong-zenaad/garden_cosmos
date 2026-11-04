package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"testing"
	"time"
)

func TestFunctionBarFixedSlots(t *testing.T) {
	u := editorFixture(t)
	u.installInput()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	root := u.screenRoot().(*appScreen)
	for _, width := range []int{80, 132} {
		s.SetSize(width, 30)
		root.SetRect(0, 0, width, 30)
		root.Draw(s)
		for i, key := range functionNames {
			x := i * width / len(functionNames)
			for n, r := range key {
				got, _, _, _ := s.GetContent(x+n, 29)
				if got != r {
					t.Fatalf("width %d slot %d: expected %s", width, i, key)
				}
			}
		}
	}
	field := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	u.app.SetFocus(field)
	labels := root.bar.labels()
	if labels[2] != "마우스 사용" || labels[3] != "마우스 해제" {
		t.Fatal(labels)
	}
	// Clicking the F6 slot uses exactly the keyboard action and keeps the footer unfocused.
	x := 5 * 132 / 11
	root.bar.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, 29, tcell.Button1, tcell.ModNone), func(tview.Primitive) {})
	if _, ok := u.app.GetFocus().(*tview.Button); !ok {
		t.Fatal("F6 click did not focus actions")
	}
	labels = root.bar.labels()
	if labels[2] != "마우스 사용" || labels[3] != "마우스 해제" {
		t.Fatal("mouse labels changed with focus", labels)
	}
}

func TestFunctionBarColorsAndExitLabel(t *testing.T) {
	u := editorFixture(t)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(132, 30)
	root := u.screenRoot().(*appScreen)
	root.SetRect(0, 0, 132, 30)
	root.Draw(s)
	_, _, style, _ := s.GetContent(0, 29)
	fg, bg, _ := style.Decompose()
	if fg != tcell.ColorWhite || bg != tcell.NewHexColor(0x222626) {
		t.Fatal("key colors", fg, bg)
	}
	_, _, style, _ = s.GetContent(2, 29)
	fg, bg, _ = style.Decompose()
	if fg != tcell.ColorBlack || bg != tcell.NewHexColor(0x18b6a4) {
		t.Fatal("label colors", fg, bg)
	}
	u.clipboardMenu()
	if root.bar.labels()[9] != "종료" {
		t.Fatal("F10 missing from overlay")
	}
}

func TestF10ExitEventLoop(t *testing.T) {
	for _, busy := range []bool{false, true} {
		t.Run(fmt.Sprint(busy), func(t *testing.T) {
			u := editorFixture(t)
			u.installInput()
			screen := tcell.NewSimulationScreen("UTF-8")
			u.app.SetScreen(screen).SetRoot(u.screenRoot(), true)
			first := true
			u.app.SetAfterDrawFunc(func(tcell.Screen) {
				if !first {
					return
				}
				first = false
				go u.app.QueueUpdate(func() {
					if busy {
						u.execute("/bin/sh", []string{"-c", "sleep 30"}, []string{"test"})
					}
					u.app.QueueEvent(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
				})
			})
			done := make(chan error, 1)
			go func() { done <- u.app.Run() }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("F10 did not exit")
			}
			if u.busy {
				t.Fatal("exit did not await cancellation")
			}
		})
	}
}
