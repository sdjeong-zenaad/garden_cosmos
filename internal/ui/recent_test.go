package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestInputRecentPickerAndStableIdentity(t *testing.T) {
	u := editorFixture(t)
	e := u.editor
	hash := strings.Repeat("a", 64)
	// Use optional label input as a general hash parameter.
	e.form.GetFormItem(1).(*fieldRow).check.SetChecked(true)
	field := e.form.GetFormItem(1).(*fieldRow).FormItem.(*tview.InputField)
	field.SetText(hash)
	u.app.SetFocus(field)
	u.bookmarks()
	if len(u.store.Recent(false, "")) != 1 {
		t.Fatal("input not captured")
	}
	seen := u.store.Entries[0].LastSeen
	if err := u.rememberInputs(); err != nil {
		t.Fatal(err)
	}
	if !u.store.Entries[0].LastSeen.Equal(seen) {
		t.Fatal("unchanged input reordered history")
	}
	// Capture an output while the picker is open; the visible selection remains stable.
	address := "0x" + strings.Repeat("b", 40)
	u.store.Capture(address, nil)
	_, p := u.pages.GetFrontPage()
	outer := p.(*tview.Flex)
	box := outer.GetItem(1).(*tview.Flex)
	layout := box.GetItem(1).(*tview.Flex)
	list := layout.GetItem(1).(*tview.List)
	field.SetText("")
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	if field.GetText() != hash {
		t.Fatal("inserted a different item after capture")
	}
	if u.store.Recent(false, "")[0].Value != hash {
		t.Fatal("selection did not become most recent")
	}
	var restored garden.Store
	if err := garden.Load(filepath.Join(u.dir, "bookmarks.json"), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Entries) != 2 {
		t.Fatal(restored)
	}
}
func TestAddressBookPickerFilter(t *testing.T) {
	u := editorFixture(t)
	address := "0x" + strings.Repeat("c", 40)
	u.store = garden.Store{Entries: []garden.Entry{{Value: address, Nickname: "내 지갑", LastSeen: time.Now()}, {Value: strings.Repeat("d", 64), Kind: "hash", LastSeen: time.Now()}}}
	u.entryPicker(true)
	_, p := u.pages.GetFrontPage()
	layout := p.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	list := layout.GetItem(1).(*tview.List)
	if list.GetItemCount() != 1 {
		t.Fatal(list.GetItemCount())
	}
	search := layout.GetItem(0).(*tview.InputField)
	search.SetText("지갑")
	if list.GetItemCount() != 1 {
		t.Fatal("alias not searchable")
	}
	search.SetText("missing")
	if list.GetItemCount() != 0 {
		t.Fatal("search not applied")
	}
}
