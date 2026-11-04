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
	unwrapFormItem(e.form.GetFormItem(1)).(*fieldRow).check.SetChecked(true)
	field := unwrapFormItem(e.form.GetFormItem(1)).(*fieldRow).FormItem.(*tview.InputField)
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

func TestEmptyAddressCompletionAndBlurCapture(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	older := "0x" + strings.Repeat("a", 40)
	newest := "0x" + strings.Repeat("b", 40)
	hash := strings.Repeat("c", 64)
	u.store.Capture(older+" "+hash+" "+newest, nil)
	u.selected = &garden.Command{Usage: "demo [to_address] [txhash]", Path: []string{"query"}}
	u.commandForm()
	address := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	hashInput := unwrapFormItem(u.editor.form.GetFormItem(1)).(*tview.InputField)
	u.app.SetFocus(address)
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()
	screen.SetSize(120, 40)
	root := u.screenRoot()
	root.SetRect(0, 0, 120, 40)
	root.Draw(screen)
	got := u.editor.autocomplete[address]("")
	if len(got) != 2 || got[0] != newest {
		t.Fatal(got)
	}
	address.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if address.GetText() != newest {
		t.Fatal("did not choose newest address", address.GetText())
	}
	fresh := strings.Repeat("d", 64)
	u.app.SetFocus(hashInput)
	hashInput.SetText(fresh)
	u.app.SetFocus(address)
	var saved garden.Store
	if err := garden.Load(filepath.Join(u.dir, "bookmarks.json"), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Recent(false, "")[0].Value != fresh {
		t.Fatal("input not cached on blur")
	}
	got = u.editor.autocomplete[hashInput]("")
	if len(got) != 2 || got[0] != fresh {
		t.Fatal(got)
	}
}

func TestCompletionAliasDisplayAndValueOnly(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	address := "xpla1zapp6g36m0j7gcsmr2n8l6yhmgw3es2pcte3tz"
	hash := strings.Repeat("a", 64)
	u.store.Put(address, "address", "운영 [red] 지갑", "")
	u.store.Put(hash, "hash", "입금 거래", "")
	u.selected = &garden.Command{Usage: "demo [address] [txhash]", Path: []string{"query"}}
	u.commandForm()
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()
	screen.SetSize(140, 40)
	root := u.screenRoot()
	root.SetRect(0, 0, 140, 40)
	for i, tc := range []struct{ value, alias string }{{address, "운영 [red] 지갑"}, {hash, "입금 거래"}} {
		input := unwrapFormItem(u.editor.form.GetFormItem(i)).(*tview.InputField)
		u.app.SetFocus(input)
		root.Draw(screen)
		got := u.editor.autocomplete[input]("")
		if len(got) != 1 || got[0] != tc.value+" · "+tc.alias {
			t.Fatal(got)
		}
		_, y, _, _ := input.GetRect()
		found := false
		for x := 0; x < 140; x++ {
			r, _, style, _ := screen.GetContent(x, y+1)
			if r == rune(tc.value[0]) {
				fg, bg, _ := style.Decompose()
				if fg == tcell.NewHexColor(0x20262e) && bg == tcell.NewHexColor(0xe6edf3) {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatal("light selected background/dark text not rendered")
		}
		input.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		if input.GetText() != tc.value {
			t.Fatal("alias leaked into input", input.GetText())
		}
	}
}
