package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Canonicalize InputField's internal TextArea focus for clipboard buttons.
func (u *UI) modeFocus() tview.Primitive {
	_, page := u.pages.GetFrontPage()
	if field := focusedInput(page); field != nil {
		return field
	}
	return u.app.GetFocus()
}

// Called only from event handlers, never under the application's draw lock.
func (u *UI) setTerminalMode(enabled bool) {
	changed := u.terminalMode != enabled
	u.terminalMode = enabled
	u.app.EnableMouse(!enabled)
	if changed && !enabled {
		u.app.QueueEvent(tcell.NewEventMouse(-1, -1, tcell.ButtonNone, tcell.ModNone))
	}
}
