package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type appScreen struct {
	*tview.Flex
	ui  *UI
	bar *functionBar
}

func (u *UI) screenRoot() tview.Primitive {
	bar := &functionBar{Box: tview.NewBox(), ui: u}
	return &appScreen{tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.pages, 0, 1, true).AddItem(bar, 1, 0, false), u, bar}
}
func (s *appScreen) Draw(screen tcell.Screen) {
	_, page := s.ui.pages.GetFrontPage()
	// InputField's mouse handler holds its autocomplete mutex while it
	// changes focus. Refresh only after that handler has returned, during draw.
	if ed := s.ui.editor; ed != nil && ed.pendingCompletion != nil {
		field := ed.pendingCompletion
		ed.pendingCompletion = nil
		if field.HasFocus() {
			field.Autocomplete()
		}
	}
	decorateButtons(page)
	s.Flex.Draw(screen)
	visitWidgets(page, func(p tview.Primitive) {
		if drop, ok := p.(*tview.DropDown); ok && drop.HasFocus() && drop.IsOpen() {
			drawComboOverlay(screen, drop)
		}
	})
	s.bar.Draw(screen)
}

var functionKeys = []tcell.Key{tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8, tcell.KeyF9, tcell.KeyF10, tcell.KeyF12}
var functionNames = []string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F12"}

type functionBar struct {
	*tview.Box
	ui *UI
}

func (b *functionBar) labels() []string {
	labels := make([]string, len(functionKeys))
	u := b.ui
	name, page := u.pages.GetFrontPage()
	switch name {
	case "main":
		labels = []string{"CLI", "주소록", "", "", "재분석", "설정", "실행", "대화형", "결과쿼리", "종료", "계산"}
	case "command":
		labels = []string{"CLI", "주소록", "", "", "", "버튼", "실행", "대화형", "", "", "계산"}
	case "clipboard":
	default:
		labels[10] = "계산"
		if u.pickerList != nil && u.pickerList.HasFocus() {
			labels[1] = "전환"
		}
	}
	if page != nil && page == u.calculatorPage {
		labels[10] = "닫기"
	}
	// Draw runs under tview's application lock. Inspect primitives directly;
	// never call Application.GetFocus (or any locking application API) here.
	if focusedInput(page) != nil {
		labels[2] = "복사"
		labels[3] = "붙임"
	}
	if u.output.HasFocus() || (u.pickerList != nil && u.pickerList.HasFocus()) {
		labels[2] = "복사"
	}
	if v := queryResultPane(page); v != nil {
		labels[8] = "돌아가기"
		if v.HasFocus() {
			labels[2] = "복사"
		}
	}
	if u.terminalClipboard {
		labels[2], labels[3] = "", ""
	}
	labels[0] = "CLI"
	labels[9] = "종료"
	return labels
}
func focusedInput(p tview.Primitive) *tview.InputField {
	switch v := p.(type) {
	case *tview.InputField:
		if v.HasFocus() {
			return v
		}
	case *fieldPane:
		return focusedInput(v.Form)
	case *fieldRow:
		return focusedInput(v.FormItem)
	case *commandPage:
		return focusedInput(v.Flex)
	case *tview.Flex:
		for i := 0; i < v.GetItemCount(); i++ {
			if f := focusedInput(v.GetItem(i)); f != nil {
				return f
			}
		}
	case *tview.Form:
		for i := 0; i < v.GetFormItemCount(); i++ {
			if f := focusedInput(v.GetFormItem(i)); f != nil {
				return f
			}
		}
	}
	return nil
}
func (b *functionBar) Draw(screen tcell.Screen) {
	b.Box.DrawForSubclass(screen, b)
	x, y, w, _ := b.GetInnerRect()
	labels := b.labels()
	for i, key := range functionNames {
		left, right := x+i*w/len(functionKeys), x+(i+1)*w/len(functionKeys)
		keyStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewHexColor(0x222626))
		labelStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.NewHexColor(0x18b6a4))
		for col := left; col < right; col++ {
			style := labelStyle
			if col < left+len(key) {
				style = keyStyle
			}
			screen.SetContent(col, y, ' ', nil, style)
		}
		tview.Print(screen, key, left, y, right-left, tview.AlignLeft, tcell.ColorWhite)
		label := labels[i]
		if label == "붙임" && right-left-len(key) >= 8 {
			label = "붙여넣기"
		}
		tview.Print(screen, label, left+len(key), y, max(0, right-left-len(key)), tview.AlignLeft, tcell.ColorBlack)
	}
}
func (b *functionBar) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return b.WrapMouseHandler(func(a tview.MouseAction, e *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := e.Position()
		if a != tview.MouseLeftClick || !b.InRect(x, y) {
			return false, nil
		}
		bx, _, w, _ := b.GetInnerRect()
		for i := range functionKeys {
			if x >= bx+i*w/len(functionKeys) && x < bx+(i+1)*w/len(functionKeys) && b.labels()[i] != "" {
				event := tcell.NewEventKey(functionKeys[i], 0, tcell.ModNone)
				if capture := b.ui.app.GetInputCapture(); capture != nil {
					event = capture(event)
				}
				if event != nil {
					b.ui.pages.InputHandler()(event, func(p tview.Primitive) { b.ui.app.SetFocus(p) })
				}
				break
			}
		}
		return true, nil
	})
}
func decorateButtons(p tview.Primitive) {
	visitWidgets(p, func(p tview.Primitive) {
		switch v := p.(type) {
		case *tview.Button:
			styleButton(v)
		case *tview.DropDown:
			v.SetListStyles(comboNormal, comboSelected)
		}
	})
}

func visitWidgets(p tview.Primitive, visit func(tview.Primitive)) {
	visit(p)
	switch v := p.(type) {
	case *actionModal:
		visitWidgets(v.Flex, visit)
	case *commandPage:
		visitWidgets(v.Flex, visit)
	case *fieldPane:
		visitWidgets(v.Form, visit)
	case *fieldRow:
		visitWidgets(v.FormItem, visit)
	case *tview.Flex:
		for i := 0; i < v.GetItemCount(); i++ {
			visitWidgets(v.GetItem(i), visit)
		}
	case *tview.Form:
		for i := 0; i < v.GetFormItemCount(); i++ {
			visitWidgets(v.GetFormItem(i), visit)
		}
		for i := 0; i < v.GetButtonCount(); i++ {
			visitWidgets(v.GetButton(i), visit)
		}
	}
}
