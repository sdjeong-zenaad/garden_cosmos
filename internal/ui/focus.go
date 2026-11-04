package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// One Tab policy for pages, forms, and inline actions. Pages declare their
// semantic order; visibility and enabled state are evaluated on each keypress.
func (u *UI) setFocusOrder(page tview.Primitive, order func() []tview.Primitive) {
	if u.focusOrders == nil {
		u.focusOrders = map[tview.Primitive]func() []tview.Primitive{}
	}
	u.focusOrders[page] = order
}
func formTargets(form *tview.Form) []tview.Primitive {
	var items []tview.Primitive
	for i := 0; i < form.GetFormItemCount(); i++ {
		item := unwrapFormItem(form.GetFormItem(i))
		if row, ok := item.(*fieldRow); ok {
			items = append(items, row.check)
			if row.check.IsChecked() {
				items = append(items, row.FormItem)
			}
		} else {
			items = append(items, item)
		}
	}
	for i := 0; i < form.GetButtonCount(); i++ {
		if !form.GetButton(i).IsDisabled() {
			items = append(items, form.GetButton(i))
		}
	}
	return items
}
func (u *UI) focusTargets(page tview.Primitive) []tview.Primitive {
	var order []tview.Primitive
	if get := u.focusOrders[page]; get != nil {
		order = get()
	} else {
		var walk func(tview.Primitive)
		walk = func(p tview.Primitive) {
			switch v := p.(type) {
			case *actionModal:
				walk(v.Flex)
			case *commandPage:
				walk(v.Flex)
			case *fieldPane:
				order = append(order, formTargets(v.Form)...)
			case *tview.Form:
				order = append(order, formTargets(v)...)
			case *tview.Flex:
				for i := 0; i < v.GetItemCount(); i++ {
					walk(v.GetItem(i))
				}
			case *edgeButton:
				order = append(order, v.Button)
			case *tview.Button:
				if !v.IsDisabled() {
					order = append(order, v)
				}
			case *tview.InputField, *tview.List, *queryView:
				order = append(order, p)
			}
		}
		walk(page)
	}
	var expanded []tview.Primitive
	for _, p := range order {
		expanded = append(expanded, p)
		if u.screen != nil {
			for _, c := range u.screen.controls {
				if c.target == p {
					expanded = append(expanded, c.button)
				}
			}
		}
	}
	return expanded
}
func (u *UI) cycleFocus(page tview.Primitive, key tcell.Key) bool {
	if key != tcell.KeyTab && key != tcell.KeyBacktab {
		return false
	}
	if page == nil {
		_, page = u.pages.GetFrontPage()
	}
	order := u.focusTargets(page)
	if len(order) == 0 {
		return false
	}
	current := u.app.GetFocus()
	index := -1
	for i, p := range order {
		if p == current {
			index = i
			break
		}
	}
	if index < 0 {
		canonical := u.modeFocus()
		for i, p := range order {
			if p == canonical {
				index = i
				break
			}
		}
	}
	next := 0
	if key == tcell.KeyBacktab {
		if index < 0 {
			next = len(order) - 1
		} else {
			next = (index - 1 + len(order)) % len(order)
		}
	} else {
		next = (index + 1) % len(order)
	}
	u.app.SetFocus(order[next])
	return true
}

// Form sees an inline button as belonging to its input row for scrolling and
// highlighting, without actually giving the input keyboard focus as well.
type focusFormItem struct {
	tview.FormItem
	ui *UI
}

func (f *focusFormItem) Focus(set func(tview.Primitive)) { set(f.FormItem) }

// Only the row's actual input receives paste events; an inline action may
// keep the row visible without making that input editable.
func (f *focusFormItem) PasteHandler() func(string, func(tview.Primitive)) {
	return func(text string, setFocus func(tview.Primitive)) {
		if f.FormItem.HasFocus() {
			if handler := f.FormItem.PasteHandler(); handler != nil {
				handler(text, setFocus)
			}
		}
	}
}
func (f *focusFormItem) HasFocus() bool {
	if f.FormItem.HasFocus() {
		return true
	}
	if f.ui.screen == nil {
		return false
	}
	target := tview.Primitive(f.FormItem)
	if row, ok := f.FormItem.(*fieldRow); ok {
		target = row.FormItem
	}
	for key, b := range f.ui.screen.controlButtons {
		if key.target == target && b.HasFocus() {
			return true
		}
	}
	return false
}
func unwrapFormItem(item tview.FormItem) tview.FormItem {
	if f, ok := item.(*focusFormItem); ok {
		return f.FormItem
	}
	return item
}
func (u *UI) bindFormFocus(form *tview.Form) {
	if form.GetFormItemCount() == 0 {
		return
	}
	if _, ok := form.GetFormItem(0).(*focusFormItem); ok {
		return
	}
	items := make([]tview.FormItem, form.GetFormItemCount())
	for i := range items {
		items[i] = form.GetFormItem(i)
	}
	form.Clear(false)
	for _, item := range items {
		form.AddFormItem(&focusFormItem{item, u})
	}
}

func (u *UI) forgetPageFocus(name string) {
	page := u.pages.GetPage(name)
	delete(u.focusOrders, page)
}
func (u *UI) repairFocus() {
	from, target := u.focusRepairFrom, u.focusRepairTarget
	u.focusRepairFrom = nil
	u.focusRepairTarget = nil
	u.focusRepairQueued = false
	if from != nil && u.app.GetFocus() == from {
		u.app.SetFocus(target)
	}
}

// Draw must never acquire the application's focus mutex. Queue a repair for the
// event loop if resizing or a popup has hidden the currently focused action.
func (u *UI) requestFocusRepair(from *tview.Button, target tview.Primitive) {
	u.focusRepairFrom, u.focusRepairTarget = from, target
	if u.launchTask != nil && u.postTask != nil && !u.focusRepairQueued {
		u.focusRepairQueued = true
		u.launchTask(func() { u.postTask(u.repairFocus) })
	}
}
