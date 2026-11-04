package ui

import (
	"strings"
	"testing"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestScreenAndCalculator(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if e := screen.Init(); e != nil {
		t.Fatal(e)
	}
	defer screen.Fini()
	screen.SetSize(140, 45)
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages(), dir: t.TempDir()}
	u.build()
	u.pages.SetRect(0, 0, 140, 45)
	u.descriptor = &garden.Descriptor{Root: &garden.Command{Path: []string{"query"}, Summary: "query", Help: "query help"}}
	u.populate("")
	u.render(`{"balance":"1000000000000000001","code":0}`, "")
	u.pages.Draw(screen)
	screen.Show()
	cells, w, h := screen.GetContents()
	var text strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			text.WriteString(string(cells[y*w+x].Runes))
		}
		text.WriteByte('\n')
	}
	if !strings.Contains(text.String(), "1000000000000000001") || !strings.Contains(text.String(), "COMMANDS") {
		t.Fatal("main screen content missing")
	}
	input := tview.NewInputField()
	u.editor = &commandEditor{form: tview.NewForm(), active: input, activeKey: "arg:0", values: map[string]garden.FieldValue{"arg:0": {Enabled: true}}}
	u.calculator()
	u.pages.Draw(screen)
	screen.Show()
	cells, w, h = screen.GetContents()
	text.Reset()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			text.WriteString(string(cells[y*w+x].Runes))
		}
		text.WriteByte('\n')
	}
	if !strings.Contains(strings.ReplaceAll(text.String(), " ", ""), "토큰저장") {
		t.Fatal("calculator buttons missing", text.String())
	}
	_, primitive := u.pages.GetFrontPage()
	if primitive == nil {
		t.Fatal("calculator missing")
	}
	// The modal's centered flex wraps the calculator form.
	outer := primitive.(*tview.Flex)
	box := outer.GetItem(1).(*tview.Flex)
	layout := box.GetItem(1).(*tview.Flex)
	form := layout.GetItem(0).(*tview.Form)
	unwrapFormItem(form.GetFormItem(0)).(*tview.DropDown).SetCurrentOption(2)
	unwrapFormItem(form.GetFormItem(4)).(*tview.InputField).SetText("1.000000000000000001")
	unwrapFormItem(form.GetFormItem(5)).(*tview.DropDown).SetCurrentOption(1)
	form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	result := layout.GetItem(1).(*tview.TextView).GetText(true)
	if !strings.Contains(result, "1000000000000000001") {
		t.Fatal(result)
	}
	form.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	if input.GetText() != "1000000000000000001axpla" {
		t.Fatal(input.GetText())
	}
}
func TestColorEscapes(t *testing.T) {
	s := colorOutput(`{"key":"[red]hello","n":1}`)
	if !strings.Contains(s, "[red[]hello") || !strings.Contains(s, "[cyan]1") {
		t.Fatal(s)
	}
}

func TestSearchListKeyboardFocus(t *testing.T) {
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages(), dir: t.TempDir()}
	u.build()
	u.app.SetRoot(u.pages, true)
	if u.app.GetFocus() != u.search {
		t.Fatal("initial focus is not search")
	}
	u.installInput()
	for _, want := range []tview.Primitive{u.output, u.list, u.search} {
		u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
		if u.app.GetFocus() != want {
			t.Fatal("reverse focus order mismatch")
		}
	}
	for _, want := range []tview.Primitive{u.list, u.output, u.search} {
		u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		if u.app.GetFocus() != want {
			t.Fatal("forward focus order changed")
		}
	}
	focus := func(p tview.Primitive) { u.app.SetFocus(p) }
	u.list.AddItem("first", "", 0, nil).AddItem("last", "", 0, nil)
	u.search.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), focus)
	if u.list.GetCurrentItem() != 1 || u.app.GetFocus() != u.list {
		t.Fatal("up from search did not select last item")
	}
	u.list.Clear()
	for _, key := range []tcell.Key{tcell.KeyUp, tcell.KeyDown} {
		u.app.SetFocus(u.search)
		u.search.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), focus)
		if u.app.GetFocus() != u.list {
			t.Fatal("arrow did not focus list")
		}
		u.list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, '가', tcell.ModNone), focus)
		if u.app.GetFocus() != u.search {
			t.Fatal("text did not focus search")
		}
	}
	if u.search.GetText() != "가가" {
		t.Fatalf("text lost or repeated: %q", u.search.GetText())
	}
	u.app.SetFocus(u.list)
	u.list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt), focus)
	if u.app.GetFocus() != u.list || u.search.GetText() != "가가" {
		t.Fatal("modified shortcut became search text")
	}
}

func TestCommandBrowserOmitsGroups(t *testing.T) {
	send := &garden.Command{Path: []string{"tx", "bank", "send"}, Usage: "demo tx bank send [from] [to] [amount] [flags]"}
	bank := &garden.Command{Path: []string{"tx", "bank"}, Usage: "demo tx bank [command]", Children: []*garden.Command{send}}
	tx := &garden.Command{Path: []string{"tx"}, Usage: "demo tx [command]", Children: []*garden.Command{bank}}
	block := &garden.Command{Path: []string{"query", "block"}, Usage: "demo query block [height]"}
	query := &garden.Command{Path: []string{"query"}, Children: []*garden.Command{block}}
	root := &garden.Command{Children: []*garden.Command{query, tx, {Path: []string{"empty-group"}, Usage: "demo empty-group [command]"}}}
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages()}
	u.build()
	u.descriptor = &garden.Descriptor{Root: root}
	u.populate("")
	if u.list.GetItemCount() != 2 || u.selected != block {
		t.Fatal("groups still visible or selection wrong")
	}
	u.search.SetText("tx bank")
	if u.list.GetItemCount() != 1 || u.selected != send {
		t.Fatal("nested executable not searchable")
	}
	u.search.SetText("empty-group")
	if u.list.GetItemCount() != 0 || u.selected != nil {
		t.Fatal("help-only group still searchable")
	}
	if len(root.Children) != 3 || len(bank.Children) != 1 {
		t.Fatal("descriptor modified")
	}
}
