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
	u.sshInputModes = remote
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

func TestSSHInputModeTransitions(t *testing.T) {
	u, root, s, first, second := modeFixture(t, true)
	check := func(terminal bool) {
		t.Helper()
		if u.terminalMode != terminal || s.mouse == terminal {
			t.Fatalf("terminal=%v mouse=%v want terminal=%v", u.terminalMode, s.mouse, terminal)
		}
	}
	check(false)
	modeKey(u, root, tcell.KeyRune, 'x') // rejected integer
	check(false)
	modeKey(u, root, tcell.KeyBackspace2, 0) // no deletion
	check(false)
	first.SetText("1") // restored/programmatic text
	root.Draw(s)
	check(false)
	modeKey(u, root, tcell.KeyRune, '2')
	check(true)
	if root.bar.labels()[2] != "앱모드" {
		t.Fatal("missing terminal mode hint")
	}
	modeKey(u, root, tcell.KeyF3, 0)
	check(false)
	modeKey(u, root, tcell.KeyDelete, 0) // cursor at end; no change
	check(false)
	modeKey(u, root, tcell.KeyBackspace2, 0)
	check(true)
	modeKey(u, root, tcell.KeyTab, 0)
	check(false)
	if !second.HasFocus() {
		t.Fatal("Tab did not leave input")
	}
	root.PasteHandler()("pasted", func(p tview.Primitive) { u.app.SetFocus(p) })
	check(true)
	modeKey(u, root, tcell.KeyF4, 0)
	check(false)
	second.SetAutocompleteFunc(func(string) []string { return []string{"choice"} })
	modeKey(u, root, tcell.KeyEnter, 0)
	check(false)
	if second.GetText() != "choice" {
		t.Fatal("completion not selected")
	}
	root.Draw(s)
	x, y, _, _ := second.GetRect()
	event, _ := u.contextMouse(tcell.NewEventMouse(x+1, y, tcell.ButtonSecondary, 0), tview.MouseRightDown)
	if event != nil {
		t.Fatal("right click not consumed")
	}
	check(true)
	modeKey(u, root, tcell.KeyF3, 0)
	check(false)
	modeKey(u, root, tcell.KeyF4, 0)
	check(true)
	modeKey(u, root, tcell.KeyF1, 0) // global handler opens another dialog
	check(false)
}

func TestLocalInputKeepsMouseAndClipboard(t *testing.T) {
	mockClipboard(t)
	u, root, s, first, _ := modeFixture(t, false)
	modeKey(u, root, tcell.KeyRune, '7')
	modeKey(u, root, tcell.KeyF3, 0)
	if u.terminalMode || !s.mouse {
		t.Fatal("local mode changed")
	}
	first.SetText("")
	modeKey(u, root, tcell.KeyF4, 0)
	if first.GetText() != "7" {
		t.Fatal("local clipboard shortcut changed", first.GetText())
	}
}

func TestSSHModeEventLoopDoesNotDeadlock(t *testing.T) {
	u, _, _, first, _ := modeFixture(t, true)
	for _, e := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, '8', 0),
		tcell.NewEventKey(tcell.KeyF3, 0, 0),
		tcell.NewEventKey(tcell.KeyF4, 0, 0),
		tcell.NewEventKey(tcell.KeyTab, 0, 0),
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
