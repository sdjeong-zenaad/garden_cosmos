package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Follow the rendered row or column within one button group. Do not let a
// button's default arrow handler turn a perpendicular arrow into Tab.
func (u *UI) moveButtonFocus(key tcell.Key) bool {
	if key != tcell.KeyLeft && key != tcell.KeyRight && key != tcell.KeyUp && key != tcell.KeyDown {
		return false
	}
	current, ok := u.app.GetFocus().(*tview.Button)
	if !ok {
		return false
	}
	_, page := u.pages.GetFrontPage()
	var group []*tview.Button
	visitWidgets(page, func(p tview.Primitive) {
		var buttons []*tview.Button
		switch v := p.(type) {
		case *tview.Form:
			for i := 0; i < v.GetButtonCount(); i++ {
				buttons = append(buttons, v.GetButton(i))
			}
		case interface {
			GetItemCount() int
			GetItem(int) tview.Primitive
		}:
			for i := 0; i < v.GetItemCount(); i++ {
				switch b := v.GetItem(i).(type) {
				case *tview.Button:
					buttons = append(buttons, b)
				case *edgeButton:
					buttons = append(buttons, b.Button)
				}
			}
		}
		for _, b := range buttons {
			if b == current {
				group = buttons
				break
			}
		}
	})
	x, y, w, h := current.GetRect()
	var next *tview.Button
	distance := int(^uint(0) >> 1)
	for _, b := range group {
		if b == current || b.IsDisabled() {
			continue
		}
		bx, by, bw, bh := b.GetRect()
		if bw <= 0 || bh <= 0 {
			continue
		}
		d := -1
		switch key {
		case tcell.KeyLeft:
			if by < y+h && by+bh > y && bx+bw <= x {
				d = x - (bx + bw)
			}
		case tcell.KeyRight:
			if by < y+h && by+bh > y && bx >= x+w {
				d = bx - (x + w)
			}
		case tcell.KeyUp:
			if bx < x+w && bx+bw > x && by+bh <= y {
				d = y - (by + bh)
			}
		case tcell.KeyDown:
			if bx < x+w && bx+bw > x && by >= y+h {
				d = by - (y + h)
			}
		}
		if d >= 0 && d < distance {
			next = b
			distance = d
		}
	}
	if next != nil {
		u.app.SetFocus(next)
	}
	return true
}
