package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Draw on the right border after content layout so wrapped lines and list
// offsets reflect the current viewport. Short or empty content fills the track.
func drawScrollbar(screen tcell.Screen, box *tview.Box, total, offset int) {
	x, y, width, height := box.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	thumb := min(height, max(1, height*height/max(1, total)))
	top := 0
	if total > height {
		top = min(height-thumb, max(0, offset)*(height-thumb)/(total-height))
	}
	style := tcell.StyleDefault.Foreground(tcell.ColorTurquoise).Background(box.GetBackgroundColor())
	for row := 0; row < height; row++ {
		ch := '░'
		if row >= top && row < top+thumb {
			ch = '█'
		}
		screen.SetContent(x+width, y+row, ch, nil, style)
	}
}

func (s *appScreen) drawMainScrollbars(screen tcell.Screen) {
	if name, _ := s.ui.pages.GetFrontPage(); name != "main" {
		return
	}
	u := s.ui
	row, _ := u.list.GetOffset()
	drawScrollbar(screen, u.list.Box, u.list.GetItemCount(), row)
	for _, view := range []*tview.TextView{u.help, u.output} {
		row, _ := view.GetScrollOffset()
		drawScrollbar(screen, view.Box, view.GetWrappedLineCount(), row)
	}
}

func scrollbarHit(box *tview.Box, x, y int) (row, height int, hit bool) {
	bx, by, width, height := box.GetInnerRect()
	return y - by, height, width > 0 && height > 0 && x == bx+width && y >= by && y < by+height
}

// Clicking a command's track selects a row but never executes that command.
func (s *appScreen) mainScrollbarMouse(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) bool {
	if name, _ := s.ui.pages.GetFrontPage(); name != "main" {
		return false
	}
	if action != tview.MouseLeftClick && action != tview.MouseLeftDown && action != tview.MouseLeftUp {
		return false
	}
	x, y := event.Position()
	u := s.ui
	if row, height, hit := scrollbarHit(u.list.Box, x, y); hit {
		if action == tview.MouseLeftClick {
			if count := u.list.GetItemCount(); count > 0 {
				u.list.SetCurrentItem(row * (count - 1) / max(1, height-1))
			}
			focus(u.list)
		}
		return true
	}
	for _, view := range []*tview.TextView{u.help, u.output} {
		if row, height, hit := scrollbarHit(view.Box, x, y); hit {
			if action == tview.MouseLeftClick {
				_, column := view.GetScrollOffset()
				view.ScrollTo(max(0, view.GetWrappedLineCount()-height)*row/max(1, height-1), column)
				focus(view)
			}
			return true
		}
	}
	return false
}
