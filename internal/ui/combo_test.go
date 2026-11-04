package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"testing"
)

func TestCalculatorComboOverlay(t *testing.T) {
	u := editorFixture(t)
	u.calculator()
	_, page := u.pages.GetFrontPage()
	layout := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	form := layout.GetItem(0).(*tview.Form)
	drop := unwrapFormItem(form.GetFormItem(5)).(*tview.DropDown)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 35)
	root := u.screenRoot()
	root.SetRect(0, 0, 120, 35)
	u.app.SetFocus(drop)
	root.Draw(s)
	drop.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	root.Draw(s)
	found := false
	for y := 0; y < 35; y++ {
		for x := 1; x < 120; x++ {
			r, _, style, _ := s.GetContent(x, y)
			if r != '표' {
				continue
			}
			fg, bg, _ := style.Decompose()
			if bg != comboBackground {
				continue
			}
			found = true
			// Reproduce a button label whose final wide glyph straddles
			// the popup edge, independent of terminal width/font metrics.
			s.SetContent(x-1, y, '한', nil, tcell.StyleDefault)
			drawComboOverlay(s, drop)
			first, _, _, _ := s.GetContent(x, y)
			if first != '표' {
				t.Fatal("first option character was overwritten", first)
			}
			if fg != tcell.NewHexColor(0xd8dee9) {
				t.Fatal("wrong normal foreground", fg)
			}
			_, _, _, width := s.GetContent(x-1, y)
			if width > 1 {
				t.Fatal("underlying wide glyph overlaps first popup character")
			}
			_, _, selected, _ := s.GetContent(x, y-1)
			sf, sb, _ := selected.Decompose()
			if sf != tcell.NewHexColor(0x20262e) || sb != tcell.NewHexColor(0xe6edf3) {
				t.Fatal("wrong selected palette", sf, sb)
			}
			break
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("conversion option missing or unthemed")
	}
}
