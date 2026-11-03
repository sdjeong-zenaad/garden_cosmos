package ui

import (
	"context"
	"fmt"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Operations are user-triggered, bounded to two seconds, and never execute pasted text.
func (u *UI) copyClipboard(value string) error {
	c, e := garden.DetectClipboard()
	if e != nil {
		return e
	}
	return c.Write(context.Background(), value)
}
func (u *UI) clipboardMenu() {
	name, _ := u.pages.GetFrontPage()
	if name == "clipboard" {
		return
	}
	origin := u.app.GetFocus()
	field, _ := origin.(*tview.InputField)
	if field == nil {
		_, page := u.pages.GetFrontPage()
		field = focusedInput(page)
	}
	if field == nil && name == "command" && u.editor != nil {
		field = u.editor.active
	}
	selected := ""
	hasSelected := false
	if field != nil {
		selected = field.GetText()
		hasSelected = true
	} else if origin == u.pickerList && u.pickerValue != nil {
		selected, hasSelected = u.pickerValue()
	}
	if field != nil && u.editor != nil && field == u.editor.active && !u.editor.values[u.editor.activeKey].Enabled {
		field = nil
	}
	status := tview.NewTextView().SetWrap(true)
	form := tview.NewForm()
	closeMenu := func() { u.pages.RemovePage("clipboard"); u.app.SetFocus(origin) }
	copyValue := func(value string) {
		if e := u.copyClipboard(value); e != nil {
			status.SetText(e.Error())
			return
		}
		status.SetText("클립보드에 복사했습니다.")
	}
	if hasSelected {
		form.AddButton("선택 값 복사", func() { copyValue(selected) })
	}
	if u.displayOutput != "" {
		form.AddButton("표시 결과 복사", func() { copyValue(u.displayOutput) })
	}
	if u.raw != "" {
		form.AddButton("원본 stdout 복사", func() { copyValue(u.raw) })
	}
	if u.rawErr != "" {
		form.AddButton("stderr 복사", func() { copyValue(u.rawErr) })
	}
	if field != nil {
		form.AddButton("입력 칸에 붙여넣기", func() {
			c, e := garden.DetectClipboard()
			if e != nil {
				status.SetText(e.Error())
				return
			}
			text, e := c.Read(context.Background())
			if e != nil {
				status.SetText(e.Error())
				return
			}
			text, e = garden.ClipboardFieldText(text)
			if e != nil {
				status.SetText(e.Error())
				return
			}
			if text == "" {
				status.SetText("클립보드가 비어 있습니다.")
				return
			}
			before := field.GetText()
			// Suppress completion candidates during paste without losing the cursor.
			if u.editor != nil && u.editor.autocomplete[field] != nil {
				field.SetAutocompleteFunc(func(string) []string { return nil })
				defer field.SetAutocompleteFunc(u.editor.autocomplete[field])
			}
			field.PasteHandler()(text, func(tview.Primitive) {})
			if field.GetText() == before {
				status.SetText("필드 형식에 맞지 않거나 자동완성 목록이 열려 있습니다. 목록을 닫고 다시 시도하세요.")
				return
			}
			closeMenu()
			u.app.SetFocus(field)
		})
	}
	form.AddButton("닫기", closeMenu).SetBorder(true).SetTitle(" 클립보드 ")
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(form, 0, 1, true).AddItem(status, 5, 0, false)
	layout.Box = tview.NewBox()
	// Wrap buttons onto new rows in narrower terminals.
	form.SetHorizontal(true)
	layout.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape {
			closeMenu()
			return nil
		}
		return e
	})
	u.pages.AddPage("clipboard", layout, true, true)
	u.app.SetFocus(form)
}

// Right-click opens the same clipboard actions for the clicked input, preserving
// the underlying form and its draft values.
func (u *UI) contextMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if action != tview.MouseRightDown {
		return event, action
	}
	name, page := u.pages.GetFrontPage()
	if name == "clipboard" {
		return nil, action
	}
	x, y := event.Position()
	var find func(tview.Primitive) *tview.InputField
	find = func(p tview.Primitive) *tview.InputField {
		if p == nil {
			return nil
		}
		px, py, pw, ph := p.GetRect()
		if x < px || x >= px+pw || y < py || y >= py+ph {
			return nil
		}
		switch v := p.(type) {
		case *fieldPane:
			return find(v.Form)
		case *fieldRow:
			return find(v.FormItem)
		case *commandPage:
			return find(v.Flex)
		case *tview.InputField:
			return v
		case *tview.Flex:
			for i := 0; i < v.GetItemCount(); i++ {
				if f := find(v.GetItem(i)); f != nil {
					return f
				}
			}
		case *tview.Form:
			for i := 0; i < v.GetFormItemCount(); i++ {
				if f := find(v.GetFormItem(i)); f != nil {
					return f
				}
			}
		}
		return nil
	}
	if field := find(page); field != nil {
		u.app.SetFocus(field)
	}
	if u.sshInputModes {
		u.setTerminalMode(true)
		return nil, action
	}
	u.clipboardMenu()
	return nil, action
}

func (u *UI) clipboardShortcut(key tcell.Key) {
	field, _ := u.app.GetFocus().(*tview.InputField)
	_, page := u.pages.GetFrontPage()
	if field == nil {
		field = focusedInput(page)
	}
	result := queryResultPane(page)
	var err error
	if key == tcell.KeyCtrlY {
		value := ""
		switch {
		case field != nil:
			value = field.GetText()
		case u.app.GetFocus() == u.pickerList && u.pickerValue != nil:
			value, _ = u.pickerValue()
		case result != nil && result.HasFocus():
			value = result.GetText(true)
		case u.app.GetFocus() == u.output:
			value = u.displayOutput
		default:
			return
		}
		err = u.copyClipboard(value)
	} else {
		if field == nil {
			return
		}
		var c garden.Clipboard
		c, err = garden.DetectClipboard()
		if err == nil {
			var text string
			text, err = c.Read(context.Background())
			if err == nil {
				text, err = garden.ClipboardFieldText(text)
			}
			if err == nil {
				if u.editor != nil {
					if complete := u.editor.autocomplete[field]; complete != nil {
						field.SetAutocompleteFunc(func(string) []string { return nil })
						defer field.SetAutocompleteFunc(complete)
					}
				}
				before := field.GetText()
				field.PasteHandler()(text, func(tview.Primitive) {})
				if text != "" && before == field.GetText() {
					err = fmt.Errorf("필드 형식에 맞지 않는 값입니다")
				}
			}
		}
	}
	if err != nil {
		if u.editor != nil {
			u.editor.feedback.SetText(tview.Escape(err.Error()))
		} else {
			u.alert(err.Error())
		}
	}
}
