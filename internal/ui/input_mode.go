package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Mode transitions run in event handlers, never under the application's draw
// lock. Canonicalize InputField's internal TextArea focus to the owning field.
func (u *UI) modeFocus() tview.Primitive {
	_, page := u.pages.GetFrontPage()
	if field := focusedInput(page); field != nil {
		return field
	}
	return u.app.GetFocus()
}

func (u *UI) setTerminalMode(enabled bool) {
	if !u.sshInputModes || u.terminalMode == enabled {
		return
	}
	u.terminalMode = enabled
	u.modeOrigin = nil
	if enabled {
		u.modeOrigin = u.modeFocus()
	}
	u.app.EnableMouse(!enabled)
	if !enabled {
		// The terminal may have consumed the release of the right click that
		// entered terminal mode. Clear tview's held-button state before new clicks.
		u.app.QueueEvent(tcell.NewEventMouse(-1, -1, tcell.ButtonNone, tcell.ModNone))
	}
}

func (u *UI) syncInputMode() {
	if u.sshInputModes && u.terminalMode && u.modeFocus() != u.modeOrigin {
		u.setTerminalMode(false)
	}
}

func editingKey(e *tcell.EventKey) bool {
	switch e.Key() {
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete,
		tcell.KeyCtrlD, tcell.KeyCtrlK, tcell.KeyCtrlW, tcell.KeyCtrlU,
		tcell.KeyCtrlX, tcell.KeyCtrlV, tcell.KeyCtrlZ, tcell.KeyCtrlY:
		return true
	}
	return false
}

func (s *appScreen) observeEdit(edit bool, handle func()) {
	u := s.ui
	_, page := u.pages.GetFrontPage()
	field := focusedInput(page)
	before := ""
	if field != nil {
		before = field.GetText()
	}
	handle()
	u.syncInputMode()
	if u.sshInputModes && edit && field != nil && field.GetText() != before && u.modeFocus() == field {
		u.setTerminalMode(true)
	}
}

func (s *appScreen) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	handler := s.Flex.InputHandler()
	return func(e *tcell.EventKey, focus func(tview.Primitive)) {
		s.observeEdit(editingKey(e), func() { handler(e, focus) })
	}
}

func (s *appScreen) PasteHandler() func(string, func(tview.Primitive)) {
	handler := s.Flex.PasteHandler()
	return func(text string, focus func(tview.Primitive)) {
		s.observeEdit(true, func() { handler(text, focus) })
	}
}

func (s *appScreen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	handler := s.Flex.MouseHandler()
	return func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		defer s.ui.syncInputMode()
		return handler(action, event, focus)
	}
}
