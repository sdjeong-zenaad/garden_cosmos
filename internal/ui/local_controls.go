package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var focusedPaneBackground = tcell.NewHexColor(0x303638)

type controlKey struct {
	target tview.Primitive
	label  string
}

type localControl struct {
	button *tview.Button
	target tview.Primitive
	action func()
	cells  []controlCell
}

type controlCell struct {
	rune  rune
	style tcell.Style
}

// Controls are registered only by widgets actually drawn in the current viewport.
func (s *appScreen) drawControl(screen tcell.Screen, x, y, width int, label string, target tview.Primitive, action func()) {
	key := controlKey{target, label}
	button := s.controlButtons[key]
	if button == nil {
		button = tview.NewButton(label)
		styleButton(button)
		s.controlButtons[key] = button
	}
	button.SetRect(x, y, width, 1)
	button.Draw(screen)
	cells := make([]controlCell, width)
	for i := range cells {
		r, _, style, _ := screen.GetContent(x+i, y)
		cells[i] = controlCell{r, style}
	}
	s.controls = append(s.controls, localControl{button, target, action, cells})
}

func (s *appScreen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(a tview.MouseAction, e *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		_, page := s.ui.pages.GetFrontPage()
		x, y := e.Position()
		for i := len(s.controls) - 1; i >= 0; i-- {
			c := s.controls[i]
			if !c.button.InRect(x, y) {
				continue
			}
			// Ignore geometry left over from a page that has just closed.
			present := false
			visitWidgets(page, func(p tview.Primitive) {
				if p == c.target {
					present = true
				}
			})
			if !present || !s.ui.hasClipboard() {
				continue
			}
			if a == tview.MouseLeftClick {
				focus(c.target)
				c.action()
			}
			if a == tview.MouseLeftDown || a == tview.MouseLeftUp || a == tview.MouseLeftClick {
				return true, nil
			}
		}
		return s.Flex.MouseHandler()(a, e, focus)
	}
}

func (s *appScreen) decorateLocalControls(page tview.Primitive) {
	disabled := map[*tview.InputField]bool{}
	labelWidths := map[*tview.InputField]int{}
	visitWidgets(page, func(p tview.Primitive) {
		if form, ok := p.(*tview.Form); ok {
			s.ui.bindFormFocus(form)
			labelWidth := 0
			for i := 0; i < form.GetFormItemCount(); i++ {
				labelWidth = max(labelWidth, tview.TaggedStringWidth(form.GetFormItem(i).GetLabel()))
			}
			for i := 0; i < form.GetFormItemCount(); i++ {
				item := unwrapFormItem(form.GetFormItem(i))
				width := labelWidth + 1
				if row, ok := item.(*fieldRow); ok {
					item = row.FormItem
					width -= 4
				}
				if field, ok := item.(*tview.InputField); ok {
					labelWidths[field] = width
				}
			}
		}
		if row, ok := p.(*fieldRow); ok {
			if field, ok := row.FormItem.(*tview.InputField); ok {
				disabled[field] = !row.check.IsChecked()
			}
		}
	})
	visitWidgets(page, func(p tview.Primitive) {
		if box, ok := p.(interface{ SetTitleAlign(int) *tview.Box }); ok {
			box.SetTitleAlign(tview.AlignLeft)
		}
		switch v := p.(type) {
		case *tview.List:
			var copyText func() string
			if v != s.ui.list {
				copyText = func() string {
					var lines []string
					for i := 0; i < v.GetItemCount(); i++ {
						main, _ := v.GetItemText(i)
						lines = append(lines, main)
					}
					return strings.Join(lines, "\n")
				}
			}
			s.decoratePane(v.Box, v, copyText)
			bg := v.GetBackgroundColor()
			v.SetMainTextStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor).Background(bg))
			v.SetSecondaryTextStyle(tcell.StyleDefault.Foreground(tview.Styles.TertiaryTextColor).Background(bg))
			v.SetShortcutStyle(tcell.StyleDefault.Foreground(tview.Styles.SecondaryTextColor).Background(bg))
		case *tview.TextView:
			if v.GetTitle() != "" {
				s.decoratePane(v.Box, v, func() string { return v.GetText(true) })
			}
		case *queryView:
			s.decoratePane(v.Box, v, func() string { return v.GetText(true) })
		case *fieldPane:
			bg := tview.Styles.PrimitiveBackgroundColor
			if v.HasFocus() {
				bg = focusedPaneBackground
			}
			v.SetBackgroundColor(bg)
		case *tview.Form:
			bg := tview.Styles.PrimitiveBackgroundColor
			if v.HasFocus() {
				bg = focusedPaneBackground
			}
			v.SetBackgroundColor(bg)
		case *tview.DropDown:
			// Form attributes update the normal field style on each draw, but
			// disabled options retain a separate style.
			v.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
				v.SetDisabledStyle(tcell.StyleDefault.Foreground(tview.Styles.SecondaryTextColor).Background(v.GetBackgroundColor()))
				return x, y, w, h
			})
		case *tview.InputField:
			enabled := !disabled[v]
			// Use the row's available width; the draw hook reserves space for actions.
			v.SetFieldWidth(0)
			v.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
				const reserved = 22 // one space + [ copy ] + one space + [ paste ]
				if !enabled || !s.clipboardAvailable || h < 1 || w < max(labelWidths[v], tview.TaggedStringWidth(v.GetLabel()))+reserved+8 {
					return x, y, w, h
				}
				s.drawControl(screen, x+w-21, y, 8, "복사", v, func() { s.ui.clipboardAction(tcell.KeyCtrlY) })
				s.drawControl(screen, x+w-12, y, 12, "붙여넣기", v, func() { s.ui.clipboardAction(tcell.KeyCtrlV) })
				return x, y, w - reserved, h
			})
		}
	})
}

func (s *appScreen) decoratePane(box *tview.Box, target tview.Primitive, text func() string) {
	bg := tview.Styles.PrimitiveBackgroundColor
	if target.HasFocus() {
		bg = focusedPaneBackground
	}
	// TextView also keeps a text background; use its override, not Box alone.
	if pane, ok := target.(interface{ SetBackgroundColor(tcell.Color) *tview.Box }); ok {
		pane.SetBackgroundColor(bg)
	}
	box.SetTitleAlign(tview.AlignLeft)
	box.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		if text != nil && s.clipboardAvailable && w >= 14 && h >= 2 {
			// Redraw the title with room reserved for the right-aligned copy button.
			border := tview.Borders.Horizontal
			if target.HasFocus() {
				border = tview.Borders.HorizontalFocus
			}
			for col := x + 1; col < x+w-1; col++ {
				screen.SetContent(col, y, border, nil, tcell.StyleDefault.Foreground(box.GetBorderColor()).Background(bg))
			}
			tview.Print(screen, box.GetTitle(), x+1, y, w-12, tview.AlignLeft, tview.Styles.TitleColor)
			s.drawControl(screen, x+w-9, y, 8, "복사", target, func() {
				if err := s.ui.copyClipboard(text()); err != nil {
					s.ui.alert(err.Error())
				}
			})
		}
		return x + 1, y + 1, max(0, w-2), max(0, h-2)
	})
}

// A completion popup may cover controls belonging to another row. Such controls
// must not intercept clicks intended for the popup.
func (s *appScreen) retainVisibleControls(screen tcell.Screen) {
	_, page := s.ui.pages.GetFrontPage()
	present := map[tview.Primitive]bool{}
	visitWidgets(page, func(p tview.Primitive) { present[p] = true })
	for key := range s.controlButtons {
		if !present[key.target] {
			delete(s.controlButtons, key)
		}
	}
	visible := s.controls[:0]
	for _, c := range s.controls {
		if !present[c.target] {
			continue
		}
		x, y, _, _ := c.button.GetRect()
		matches := true
		for i, cell := range c.cells {
			r, _, style, _ := screen.GetContent(x+i, y)
			if r != cell.rune || style != cell.style {
				matches = false
				break
			}
		}
		if matches {
			visible = append(visible, c)
		}
	}
	s.controls = visible
	for key, b := range s.controlButtons {
		if !b.HasFocus() {
			continue
		}
		found := false
		for _, c := range visible {
			if c.button == b {
				found = true
				break
			}
		}
		if !found {
			s.ui.requestFocusRepair(b, key.target)
		}
	}
}

// Local actions are outside the Form's item list. Route their keys before the
// page's normal Tab cycle, while retaining the input's logical form focus.
func (s *appScreen) localControlInput(e *tcell.EventKey) bool {
	focus := s.ui.app.GetFocus()
	current := -1
	for i, c := range s.controls {
		if c.button == focus {
			current = i
			break
		}
	}
	key := e.Key()
	if current >= 0 {
		c := s.controls[current]
		peers := []*tview.Button{}
		index := 0
		for _, p := range s.controls {
			if p.target == c.target {
				if p.button == c.button {
					index = len(peers)
				}
				peers = append(peers, p.button)
			}
		}
		switch key {
		case tcell.KeyEnter:
			_, before := s.ui.pages.GetFrontPage()
			s.ui.app.SetFocus(c.target)
			c.action()
			if _, after := s.ui.pages.GetFrontPage(); after == before {
				s.ui.app.SetFocus(c.button)
			}
			return true
		case tcell.KeyLeft, tcell.KeyRight:
			next := index - 1
			if key == tcell.KeyRight {
				next = index + 1
			}
			if next >= 0 && next < len(peers) {
				s.ui.app.SetFocus(peers[next])
			}
			return true

		}
		// Global actions still work; editing keys must not reach the retained input.
		if key == tcell.KeyEscape || key == tcell.KeyCtrlC || (key >= tcell.KeyF1 && key <= tcell.KeyF12) {
			return false
		}
		return true
	}
	return false
}
