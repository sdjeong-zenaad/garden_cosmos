package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Expose the dialog's form to the shared button styling, unlike tview.Modal's
// private form. The message remains scrollable in small terminals.
type actionModal struct {
	*tview.Flex
	text *tview.TextView
	form *tview.Form
	done func(int, string)
}

func newActionModal() *actionModal {
	m := &actionModal{Flex: tview.NewFlex().SetDirection(tview.FlexRow), text: tview.NewTextView().SetDynamicColors(true).SetWrap(true), form: tview.NewForm().SetButtonsAlign(tview.AlignCenter)}
	m.Box = tview.NewBox().SetBorder(true)
	m.AddItem(m.text, 0, 1, false).AddItem(m.form, 3, 0, true)
	m.form.SetBorderPadding(1, 0, 1, 1)
	m.form.SetCancelFunc(func() {
		if m.done != nil {
			m.done(-1, "")
		}
	})
	return m
}
func (m *actionModal) SetText(text string) *actionModal                { m.text.SetText(text); return m }
func (m *actionModal) SetDoneFunc(done func(int, string)) *actionModal { m.done = done; return m }
func (m *actionModal) AddButtons(labels []string) *actionModal {
	for i, label := range labels {
		m.form.AddButton(label, func() {
			if m.done != nil {
				m.done(i, label)
			}
		})
		b := m.form.GetButton(m.form.GetButtonCount() - 1)
		styleButton(b)
		b.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			switch e.Key() {
			case tcell.KeyLeft, tcell.KeyUp:
				return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
			case tcell.KeyRight, tcell.KeyDown:
				return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
			}
			return e
		})
	}
	return m
}
