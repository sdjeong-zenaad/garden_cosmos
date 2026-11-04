package ui

import (
	"path/filepath"
	"strings"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (u *UI) rememberInputs() error {
	ed := u.editor
	if ed == nil {
		return nil
	}
	changed := false
	for _, f := range ed.fields {
		v := ed.values[f.Key]
		if ed.recorded[f.Key] == v {
			continue
		}
		if v.Enabled {
			u.store.CaptureInput(f, v.Value, ed.command.Path)
		}
		changed = true
	}
	if !changed {
		return nil
	}
	if e := u.persistEntries(); e != nil {
		return e
	}
	if ed.recorded == nil {
		ed.recorded = map[string]garden.FieldValue{}
	}
	for k, v := range ed.values {
		ed.recorded[k] = v
	}
	return nil
}
func (u *UI) persistEntries() error {
	return garden.Save(filepath.Join(u.dir, "bookmarks.json"), u.store)
}
func (u *UI) bookmarks() {
	if e := u.rememberInputs(); e != nil {
		u.alert(e.Error())
		return
	}
	u.entryPicker(false)
}
func (u *UI) entryPicker(addressBook bool) {
	search := tview.NewInputField().SetLabel("검색 (주소/해시/별명/메모) ")
	list := tview.NewList().ShowSecondaryText(true)
	title := " 캐시 전체 · 최신순 · Enter로 입력 "
	if addressBook {
		title = " 주소록 · 별명으로 검색 · Enter로 입력 "
	}
	list.SetBorder(true).SetTitle(title)
	info := tview.NewTextView().SetText("명령 폼의 활성 입력 칸에 삽입합니다. Tab: 검색/목록 · Esc: 닫기")
	entries := []garden.Entry{}
	selected := func() (garden.Entry, bool) {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(entries) {
			return garden.Entry{}, false
		}
		return entries[i], true
	}
	u.pickerList = list
	u.pickerValue = func() (string, bool) { e, ok := selected(); return e.Value, ok }
	copySelected := func() {
		if e, ok := selected(); ok {
			if err := u.copyClipboard(e.Value); err != nil {
				info.SetText(err.Error())
			} else {
				info.SetText("선택한 주소/해시를 클립보드에 복사했습니다.")
			}
		}
	}
	insert := func() {
		e, ok := selected()
		if !ok {
			return
		}
		if !u.appendArg(e.Value) {
			info.SetText("명령 폼에서 입력할 칸을 활성화하고 선택한 뒤 F2를 여세요.")
			return
		}
		u.store.Touch(e.Value)
		if err := u.persistEntries(); err != nil {
			info.SetText(err.Error())
			return
		}
		u.close()
	}
	reload := func(q string) {
		entries = u.store.Recent(addressBook, q)
		list.Clear()
		for _, e := range entries {
			label := e.Nickname
			if label == "" {
				label = e.Kind
			}
			if e.Nickname != "" {
				label = "고정 · " + label
			}
			list.AddItem(tview.Escape(label+" · "+e.Value), tview.Escape(e.LastSeen.Local().Format("2006-01-02 15:04:05")+" · "+e.Note), 0, nil)
		}
		if len(entries) == 0 {
			info.SetText("해당 항목이 없습니다. 주소 등록 버튼으로 주소록에 추가할 수 있습니다.")
		}
	}
	search.SetChangedFunc(reload)
	list.SetSelectedFunc(func(int, string, string, rune) { insert() })
	reload("")
	buttons := tview.NewFlex()
	for _, b := range []struct {
		label string
		fn    func()
	}{{"최근", func() { u.entryPicker(false) }}, {"주소록", func() { u.entryPicker(true) }}, {"입력", insert}, {"복사", copySelected}, {"주소 등록", func() { u.editStoredEntry(garden.Entry{AddressBook: true}, addressBook) }}, {"별명/고정", func() {
		if e, ok := selected(); ok {
			u.editStoredEntry(e, addressBook)
		}
	}}, {"삭제", func() {
		if e, ok := selected(); ok {
			u.store.Delete(e.Value)
			if err := u.persistEntries(); err != nil {
				info.SetText(err.Error())
				return
			}
			reload(search.GetText())
		}
	}}, {"닫기", u.close}} {
		if b.label == "복사" && !u.hasClipboard() {
			continue
		}
		buttons.AddItem(tview.NewButton("["+b.label+"]").SetSelectedFunc(b.fn), 0, 1, false).AddItem(nil, 1, 0, false)
	}
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 1, 0, false).AddItem(list, 0, 1, true).AddItem(info, 2, 0, false).AddItem(buttons, 1, 0, false)
	layout.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch e.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			if u.app.GetFocus() == search {
				u.app.SetFocus(list)
			} else {
				u.app.SetFocus(search)
			}
			return nil
		case tcell.KeyF2:
			u.entryPicker(!addressBook)
			return nil
		case tcell.KeyCtrlN:
			u.editStoredEntry(garden.Entry{AddressBook: true}, addressBook)
			return nil
		case tcell.KeyCtrlE:
			if e, ok := selected(); ok {
				u.editStoredEntry(e, addressBook)
			}
			return nil
		case tcell.KeyDelete:
			if e, ok := selected(); ok {
				u.store.Delete(e.Value)
				if err := u.persistEntries(); err != nil {
					info.SetText(err.Error())
					return nil
				}
				reload(search.GetText())
			}
			return nil
		}
		return e
	})
	info.SetText("")
	u.modal(layout, 110, 30)
	u.app.SetFocus(list)
}
func (u *UI) editStoredEntry(entry garden.Entry, backToBook bool) {
	original := entry.Value
	newAddress := original == "" && entry.AddressBook
	f := tview.NewForm().AddInputField("주소 / 해시", entry.Value, 0, nil, nil).AddInputField("별명", entry.Nickname, 0, nil, nil).AddInputField("메모", entry.Note, 0, nil, nil)
	status := tview.NewTextView().SetWrap(true).SetText("별명을 붙이면 주소·해시가 고정되어 자동 정리에서 제외됩니다. 별명과 메모를 지우면 고정이 해제됩니다. 주소는 주소록에도 표시됩니다.")
	f.AddButton("저장", func() {
		entry.Value = strings.TrimSpace(f.GetFormItem(0).(*tview.InputField).GetText())
		entry.Nickname = f.GetFormItem(1).(*tview.InputField).GetText()
		entry.Note = f.GetFormItem(2).(*tview.InputField).GetText()
		entry.AddressBook = newAddress || (garden.IsAddress(entry.Value) && strings.TrimSpace(entry.Nickname) != "")
		if garden.IsAddress(entry.Value) {
			entry.Kind = "address"
		}
		if e := u.store.Edit(original, entry); e != nil {
			status.SetText(e.Error())
			return
		}
		if e := u.persistEntries(); e != nil {
			status.SetText(e.Error())
			return
		}
		u.entryPicker(backToBook)
	}).AddButton("취소", func() { u.entryPicker(backToBook) })
	f.SetBorder(true).SetTitle(" 주소록 / 최근 항목 편집 ")
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(f, 0, 1, true).AddItem(status, 2, 0, false)
	u.modal(panel, 100, 18)
}
