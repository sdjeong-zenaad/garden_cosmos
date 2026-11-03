package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	comboBackground = tcell.NewHexColor(0x36556b)
	comboNormal     = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xd8dee9)).Background(comboBackground)
	comboSelected   = tcell.StyleDefault.Foreground(tcell.NewHexColor(0x20262e)).Background(tcell.NewHexColor(0xe6edf3)).Bold(true)
)

func styleAutocomplete(input *tview.InputField) {
	input.SetAutocompleteUseTags(false).SetAutocompleteStyles(comboBackground, comboNormal, comboSelected)
}

// An underlying wide glyph may start just left of the popup and occupy its
// first cell. Clear that glyph after drawing the overlay so terminal renderers
// do not skip the first popup character as a wide-character continuation.
type comboScreen struct {
	tcell.Screen
	left map[int]int
}

func (s *comboScreen) SetContent(x, y int, r rune, combining []rune, style tcell.Style) {
	if left, ok := s.left[y]; !ok || x < left {
		s.left[y] = x
	}
	s.Screen.SetContent(x, y, r, combining, style)
}

func drawComboOverlay(screen tcell.Screen, drop *tview.DropDown) {
	overlay := &comboScreen{Screen: screen, left: make(map[int]int)}
	drop.Draw(overlay)
	for y, x := range overlay.left {
		if x <= 0 {
			continue
		}
		_, _, style, width := screen.GetContent(x-1, y)
		if width > 1 {
			screen.SetContent(x-1, y, ' ', nil, style)
		}
	}
}
