package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strings"
)

// Keep brackets at the physical edges, independently of the centered label.
type edgeButton struct{ *tview.Button }

func (b *edgeButton) Draw(s tcell.Screen) {
	styleButton(b.Button)
	b.Button.Draw(s)
}

// Reserve two cells on each side so labels never overwrite focus markers.
func styleButton(b *tview.Button) {
	if b.GetDrawFunc() != nil {
		return
	}
	label := b.GetLabel()
	if strings.HasPrefix(label, "[") && strings.HasSuffix(label, "]") {
		label = strings.TrimSpace(label[1 : len(label)-1])
	}
	b.SetLabel(label)
	b.SetDrawFunc(func(s tcell.Screen, x, y, w, h int) (int, int, int, int) {
		if w < 4 || h < 1 {
			return x, y, w, h
		}
		left, right := "[ ", " ]"
		if b.HasFocus() {
			left, right = "[[", "]]"
		}
		foreground := tview.Styles.PrimaryTextColor
		if b.HasFocus() && !b.IsDisabled() {
			foreground = tview.Styles.InverseTextColor
		}
		style := tcell.StyleDefault.Foreground(foreground).Background(b.GetBackgroundColor())
		for i, r := range left {
			s.SetContent(x+i, y+h/2, r, nil, style)
		}
		for i, r := range right {
			s.SetContent(x+w-2+i, y+h/2, r, nil, style)
		}
		return x + 2, y, w - 4, h
	})
}

// The thumb follows the viewport calculated by Form for the focused row.
type fieldPane struct {
	*tview.Form
	focus func(int)
}

func (p *fieldPane) Draw(s tcell.Screen) {
	p.Form.Draw(s)
	x, y, w, h := p.GetRect()
	height := h - 2
	n := p.GetFormItemCount()
	if height <= 0 || w < 2 || n == 0 {
		return
	}
	_, top, _, visible := p.GetInnerRect()
	_, first, _, _ := p.GetFormItem(0).GetRect()
	_, last, _, lh := p.GetFormItem(n - 1).GetRect()
	total := max(visible, last+lh-first)
	thumb := max(1, height*visible/total)
	offset := max(0, top-first)
	start := 0
	if total > visible {
		start = (height - thumb) * offset / (total - visible)
	}
	start = min(height-thumb, start)
	for i := 0; i < height; i++ {
		r := '░'
		if i >= start && i < start+thumb {
			r = '█'
		}
		s.SetContent(x+w-1, y+1+i, r, nil, tcell.StyleDefault.Foreground(tcell.ColorTeal))
	}
}
func (p *fieldPane) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(a tview.MouseAction, e *tcell.EventMouse, set func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := e.Position()
		px, py, w, h := p.GetRect()
		n := p.GetFormItemCount()
		if p.InRect(x, y) && n > 0 {
			i, _ := p.GetFocusedItemIndex()
			switch {
			case a == tview.MouseScrollUp:
				p.focus(max(0, i-1))
				return true, nil
			case a == tview.MouseScrollDown:
				p.focus(min(n-1, i+1))
				return true, nil
			case a == tview.MouseLeftClick && x == px+w-1 && h > 2:
				p.focus(min(n-1, max(0, (y-py-1)*n/(h-2))))
				return true, nil
			}
		}
		return p.Form.MouseHandler()(a, e, set)
	}
}
