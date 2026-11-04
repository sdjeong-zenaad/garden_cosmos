package ui

import (
	"context"
	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"testing"
)

func TestPickerClickUsesClickedEntry(t *testing.T) {
	u := editorFixture(t)
	field := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	field.SetAcceptanceFunc(nil)
	u.app.SetFocus(field)
	u.store.Put("first-value", "hash", "first", "")
	u.store.Put("second-value", "hash", "second", "")
	u.pages.SetRect(0, 0, 140, 40)
	u.entryPicker(false)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(140, 40)
	u.pages.Draw(s)
	want := u.store.Recent(false, "")[1].Value
	x, y, _, _ := u.pickerList.GetInnerRect()
	u.pickerList.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y+2, tcell.Button1, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if field.GetText() != want {
		t.Fatalf("clicked second item: got %q want %q", field.GetText(), want)
	}
}
func TestSearchDeletePreservesPinnedEntry(t *testing.T) {
	u := editorFixture(t)
	u.store.Put("saved-value", "hash", "saved", "")
	u.pages.SetRect(0, 0, 140, 40)
	u.entryPicker(false)
	_, page := u.pages.GetFrontPage()
	layout := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	search := layout.GetItem(0).(*tview.InputField)
	search.SetText("saved")
	u.app.SetFocus(search)
	page.InputHandler()(tcell.NewEventKey(tcell.KeyDelete, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if len(u.store.Entries) != 1 {
		t.Fatal("search Delete removed pinned entry")
	}
	u.app.SetFocus(u.pickerList)
	page.InputHandler()(tcell.NewEventKey(tcell.KeyDelete, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if len(u.store.Entries) != 0 {
		t.Fatal("list Delete no longer works")
	}
}
func TestProfileSwitchClosesOldEditor(t *testing.T) {
	u := editorFixture(t)
	oldKey := u.editor.preferenceKey
	field := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	field.SetText("42")
	if err := u.applyProfile(garden.Profile{Binary: "/bin/true", Home: "/new-home"}); err != nil {
		t.Fatal(err)
	}
	if u.editor != nil || u.descriptor != nil || u.selected != nil {
		t.Fatal("old editor survived profile switch")
	}
	if name, _ := u.pages.GetFrontPage(); name != "main" {
		t.Fatal("old command page survived")
	}
	if u.drafts[oldKey+"\nquery block"]["arg:0"].Value != "42" {
		t.Fatal("old draft identity lost")
	}
	u.leaveCommand() // Safe while analysis has not produced a descriptor.
}
func TestCalculatorIBCAndFactoryInsertion(t *testing.T) {
	for _, denom := range []string{"ibc/ABC", "factory/owner/token"} {
		u := editorFixture(t)
		u.leaveCommand()
		u.selected = &garden.Command{Path: []string{"send"}, Usage: "mock send [amount]"}
		u.commandForm()
		field := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
		u.app.SetFocus(field)
		u.calculator()
		form, _, _ := calculatorWidgets(u)
		unwrapFormItem(form.GetFormItem(2)).(*tview.InputField).SetText(denom)
		unwrapFormItem(form.GetFormItem(3)).(*tview.InputField).SetText("6")
		unwrapFormItem(form.GetFormItem(4)).(*tview.InputField).SetText("1")
		unwrapFormItem(form.GetFormItem(5)).(*tview.DropDown).SetCurrentOption(1)
		pressButton(form, "인자에 삽입")
		if field.GetText() != "1000000"+denom {
			t.Fatal("unit slash mistaken for fraction", field.GetText())
		}
	}
	_, _, transfer, err := calculateToken(context.Background(), "1/3", "6", "TOK", "ibc/ABC", 0, "")
	if err != nil || transfer {
		t.Fatal("recurring fraction became transferable")
	}
}
func TestCalculatorDiscardStaleJobs(t *testing.T) {
	u := editorFixture(t)
	var jobs, updates []func()
	u.launchTask = func(fn func()) { jobs = append(jobs, fn) }
	u.postTask = func(fn func()) { updates = append(updates, fn) }
	u.calculator()
	form, final, _ := calculatorWidgets(u)
	expr := unwrapFormItem(form.GetFormItem(4)).(*tview.InputField)
	expr.SetText("1")
	pressButton(form, "계산")
	if len(jobs) != 1 || final.GetText(false) != "" {
		t.Fatal("calculation was not dispatched")
	}
	jobs[0]() // Compute but delay the event-loop result until after another edit.
	expr.SetText("2")
	updates[0]()
	if final.GetText(false) != "" {
		t.Fatal("stale result overwrote edited expression")
	}
	pressButton(form, "계산")
	jobs[1]()
	updates[1]()
	if final.GetText(false) != "2" {
		t.Fatal(final.GetText(false))
	}
	pressButton(form, "계산")
	u.close()
	jobs[2]()
	if len(updates) != 2 {
		t.Fatal("closed calculator posted another update")
	}
	var saved calculatorState
	if err := garden.Load(filepath.Join(u.dir, "calculator.json"), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Expression != "2" {
		t.Fatal("close lost pending input")
	}
}
