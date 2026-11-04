package ui

import (
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type modeTestScreen struct {
	tcell.SimulationScreen
	mouse bool
	fini  sync.Once
}

func (s *modeTestScreen) Fini() { s.fini.Do(s.SimulationScreen.Fini) }

func (s *modeTestScreen) EnableMouse(...tcell.MouseFlags) { s.mouse = true }
func (s *modeTestScreen) DisableMouse()                   { s.mouse = false }

func modeFixture(t *testing.T, remote bool) (*UI, *appScreen, *modeTestScreen, *tview.InputField, *tview.InputField) {
	t.Helper()
	u := editorFixture(t)
	if remote {
		t.Setenv("SSH_CONNECTION", "remote")
	} else {
		for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
			t.Setenv(key, "")
		}
	}
	s := &modeTestScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8")}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Fini)
	s.SetSize(120, 40)
	root := u.screenRoot().(*appScreen)
	root.SetRect(0, 0, 120, 40)
	u.app.SetScreen(s).SetRoot(root, true).EnableMouse(true).EnablePaste(true)
	u.installInput()
	form := tview.NewForm().AddInputField("number", "", 20, tview.InputFieldInteger, nil).AddInputField("text", "", 20, nil, nil).AddButton("done", func() {})
	u.modal(form, 80, 20)
	first := form.GetFormItem(0).(*tview.InputField)
	second := form.GetFormItem(1).(*tview.InputField)
	u.app.SetFocus(first)
	root.Draw(s)
	return u, root, s, first, second
}
func modeKey(u *UI, root *appScreen, key tcell.Key, r rune) {
	e := u.app.GetInputCapture()(tcell.NewEventKey(key, r, tcell.ModNone))
	if e != nil {
		root.InputHandler()(e, func(p tview.Primitive) { u.app.SetFocus(p) })
	}
}

func TestFixedMouseControls(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "SSH"}[remote], func(t *testing.T) {
			u, root, s, first, _ := modeFixture(t, remote)
			check := func(terminal bool) {
				t.Helper()
				if u.terminalMode != terminal || s.mouse == terminal {
					t.Fatalf("terminal=%v mouse=%v", u.terminalMode, s.mouse)
				}
			}
			modeKey(u, root, tcell.KeyF4, 0)
			check(true)
			modeKey(u, root, tcell.KeyF4, 0)
			check(true)
			modeKey(u, root, tcell.KeyRune, '2')
			check(true)
			modeKey(u, root, tcell.KeyTab, 0)
			check(true)
			modeKey(u, root, tcell.KeyF3, 0)
			check(false)
			modeKey(u, root, tcell.KeyF3, 0)
			check(false)
			modeKey(u, root, tcell.KeyRune, 'x')
			check(false)
			if first.GetText() != "2" {
				t.Fatal("typing lost")
			}
			if root.bar.labels()[2] != "마우스 사용" || root.bar.labels()[3] != "마우스 해제" {
				t.Fatal("wrong labels")
			}
		})
	}
}

func TestSSHModeEventLoopDoesNotDeadlock(t *testing.T) {
	u, _, _, first, _ := modeFixture(t, true)
	for _, e := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, '8', 0),
		tcell.NewEventKey(tcell.KeyF3, 0, 0),
		tcell.NewEventKey(tcell.KeyF4, 0, 0),
		tcell.NewEventKey(tcell.KeyTab, 0, 0),
		tcell.NewEventKey(tcell.KeyF3, 0, 0),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, 0),
	} {
		u.app.QueueEvent(e)
	}
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("mode change deadlocked event loop")
	}
	if first.GetText() != "8" || u.terminalMode {
		t.Fatal("incorrect final mode or input")
	}
}
