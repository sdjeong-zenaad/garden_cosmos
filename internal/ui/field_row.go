package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// fieldRow keeps an optional field's switch and editor on the same line.
type fieldRow struct {
	tview.FormItem
	check      *tview.Checkbox
	x, y, w, h int
	setFocus   func(tview.Primitive)
	finished   func(tcell.Key)
	updating   bool
	backward   bool
}

func newFieldRow(item tview.FormItem, enabled bool, change func(bool), focused func(), setFocus func(tview.Primitive)) *fieldRow {
	r := &fieldRow{FormItem: item, check: tview.NewCheckbox().SetChecked(enabled).SetCheckedString("O"), setFocus: setFocus}
	r.check.SetChangedFunc(func(on bool) {
		r.updating = true
		change(on)
		r.updating = false
		if on && r.check.HasFocus() {
			r.setFocus(r.FormItem)
		}
	}).SetFocusFunc(focused)
	return r
}
func (r *fieldRow) GetLabel() string              { return "    " + r.FormItem.GetLabel() }
func (r *fieldRow) GetRect() (int, int, int, int) { return r.x, r.y, r.w, r.h }
func (r *fieldRow) SetRect(x, y, w, h int) {
	r.x, r.y, r.w, r.h = x, y, w, h
	r.check.SetRect(x, y, 3, h)
	r.FormItem.SetRect(x+4, y, max(0, w-4), h)
}
func (r *fieldRow) SetFormAttributes(w int, a, b, c, d tcell.Color) tview.FormItem {
	r.FormItem.SetFormAttributes(max(0, w-4), a, b, c, d)
	r.check.SetFormAttributes(0, a, b, c, d)
	return r
}
func (r *fieldRow) Draw(s tcell.Screen) { r.check.Draw(s); r.FormItem.Draw(s) }
func (r *fieldRow) Blur()               { r.check.Blur(); r.FormItem.Blur() }
func (r *fieldRow) HasFocus() bool      { return r.check.HasFocus() || r.FormItem.HasFocus() }
func (r *fieldRow) Focus(set func(tview.Primitive)) {
	if r.backward && r.check.IsChecked() {
		set(r.FormItem)
	} else {
		set(r.check)
	}
}
func (r *fieldRow) SetFinishedFunc(f func(tcell.Key)) tview.FormItem {
	r.finished = f
	r.check.SetFinishedFunc(func(k tcell.Key) {
		if (k == tcell.KeyTab || k == tcell.KeyEnter) && r.check.IsChecked() {
			r.setFocus(r.FormItem)
		} else if r.finished != nil {
			r.finished(k)
		}
	})
	r.FormItem.SetFinishedFunc(func(k tcell.Key) {
		if r.updating {
			return
		}
		if k == tcell.KeyBacktab {
			r.setFocus(r.check)
		} else if r.finished != nil {
			r.finished(k)
		}
	})
	return r
}
func (r *fieldRow) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return func(e *tcell.EventKey, set func(tview.Primitive)) {
		if r.check.HasFocus() {
			r.check.InputHandler()(e, set)
		} else {
			r.FormItem.InputHandler()(e, set)
		}
	}
}
func (r *fieldRow) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(a tview.MouseAction, e *tcell.EventMouse, set func(tview.Primitive)) (bool, tview.Primitive) {
		x, _ := e.Position()
		if x < r.x+4 {
			return r.check.MouseHandler()(a, e, set)
		}
		return r.FormItem.MouseHandler()(a, e, set)
	}
}
