package ui

import (
	"garden_cosmos/internal/garden"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func mockClipboard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "data")
	for name, body := range map[string]string{"wl-copy": "/bin/cat > '" + file + "'", "wl-paste": "/bin/cat '" + file + "'"} {
		if e := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); e != nil {
			t.Fatal(e)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAYLAND_DISPLAY", "mock")
	t.Setenv("DISPLAY", "")
	return file
}
func pressButton(form *tview.Form, label string) {
	for i := 0; i < form.GetButtonCount(); i++ {
		b := form.GetButton(i)
		if b.GetLabel() == label {
			b.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
			return
		}
	}
	panic("missing button " + label)
}
func clipboardForm(u *UI) *tview.Form {
	_, p := u.pages.GetFrontPage()
	return p.(*tview.Flex).GetItem(0).(*tview.Form)
}
func TestClipboardInputAndReturn(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	input := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	input.SetText("42")
	u.app.SetFocus(input)
	u.clipboardMenu()
	f := clipboardForm(u)
	pressButton(f, "선택 값 복사")
	b, e := os.ReadFile(file)
	if e != nil || string(b) != "42" {
		t.Fatal(string(b), e)
	}
	input.SetText("")
	os.WriteFile(file, []byte("123\n"), 0600)
	pressButton(f, "입력 칸에 붙여넣기")
	if input.GetText() != "123" || u.editor.values["arg:0"].Value != "123" {
		t.Fatal("paste not connected to form")
	}
	if name, _ := u.pages.GetFrontPage(); name != "command" {
		t.Fatal(name)
	}
}
func TestCopyFilteredResult(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	u.leaveCommand()
	u.raw = `{"value":42}`
	u.render("42", "")
	u.app.SetFocus(u.output)
	u.clipboardMenu()
	f := clipboardForm(u)
	pressButton(f, "표시 결과 복사")
	b, _ := os.ReadFile(file)
	if string(b) != "42" {
		t.Fatal(string(b))
	}
	pressButton(f, "원본 stdout 복사")
	b, _ = os.ReadFile(file)
	if string(b) != u.raw {
		t.Fatal(string(b))
	}
}
func TestClipboardRejectsNumericPaste(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	input := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	input.SetText("42")
	u.app.SetFocus(input)
	os.WriteFile(file, []byte("bad\n"), 0600)
	u.clipboardMenu()
	pressButton(clipboardForm(u), "입력 칸에 붙여넣기")
	if input.GetText() != "42" {
		t.Fatal("validation bypassed")
	}
	if name, _ := u.pages.GetFrontPage(); name != "clipboard" {
		t.Fatal("failure lost dialog")
	}
}

func TestRightClickInputClipboard(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	u.pages.SetRect(0, 0, 100, 30)
	u.pages.Draw(screen)
	input := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	input.SetText("4")
	u.app.SetFocus(u.editor.form.GetFormItem(1))
	x, y, _, _ := input.GetRect()
	event, _ := u.app.GetMouseCapture()(tcell.NewEventMouse(x+1, y, tcell.ButtonSecondary, tcell.ModNone), tview.MouseRightDown)
	if event != nil {
		t.Fatal("right click not consumed")
	}
	if name, _ := u.pages.GetFrontPage(); name != "clipboard" {
		t.Fatal(name)
	}
	if err := os.WriteFile(file, []byte("2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pressButton(clipboardForm(u), "입력 칸에 붙여넣기")
	if input.GetText() != "42" || u.app.GetFocus() != input {
		t.Fatal("paste target/focus lost", input.GetText())
	}
}

func TestClipboardButtons(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	u.installInput()
	field := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	field.SetText("42")
	u.app.SetFocus(field)
	clickClipboardButton(u, 0)
	b, err := os.ReadFile(file)
	if err != nil || string(b) != "42" {
		t.Fatal(string(b), err)
	}
	field.SetText("")
	clickClipboardButton(u, 1)
	if field.GetText() != "42" || u.editor.values["arg:0"].Value != "42" {
		t.Fatal("shortcut paste failed")
	}
	if name, _ := u.pages.GetFrontPage(); name != "command" {
		t.Fatal("shortcut changed page")
	}
}

func TestAddressRegistrationClipboardAfterMouseFocus(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	u.leaveCommand()
	u.installInput()
	u.pages.SetRect(0, 0, 140, 40)
	u.editStoredEntry(garden.Entry{AddressBook: true}, true)
	_, page := u.pages.GetFrontPage()
	panel := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	form := panel.GetItem(0).(*tview.Form)
	field := unwrapFormItem(form.GetFormItem(0)).(*tview.InputField)
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()
	screen.SetSize(120, 40)
	u.pages.SetRect(0, 0, 120, 40)
	u.pages.Draw(screen)
	address := "xpla1zapp6g36m0j7gcsmr2n8l6yhmgw3es2pcte3tz"
	if err := os.WriteFile(file, []byte(address+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	x, y, _, _ := field.GetRect()
	field.MouseHandler()(tview.MouseLeftDown, tcell.NewEventMouse(x+1, y, tcell.Button1, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	clickClipboardButton(u, 1)
	if field.GetText() != address {
		t.Fatalf("mouse-focused address paste failed: %q focus=%T", field.GetText(), u.app.GetFocus())
	}
	// Copy uses the same mouse-focused field, and the context menu can paste too.
	clickClipboardButton(u, 0)
	copied, err := os.ReadFile(file)
	if err != nil || string(copied) != address {
		t.Fatal("mouse-focused copy failed", err)
	}
	field.SetText("")
	u.clipboardMenu()
	pressButton(clipboardForm(u), "입력 칸에 붙여넣기")
	if field.GetText() != address {
		t.Fatal("context paste failed")
	}
	field.SetText("")
	u.pages.PasteHandler()(address, func(p tview.Primitive) { u.app.SetFocus(p) })
	if field.GetText() != address {
		t.Fatal("terminal paste failed")
	}

}

func TestTerminalClipboardFooterAndPaste(t *testing.T) {
	u := editorFixture(t)
	t.Setenv("PATH", t.TempDir())
	field := unwrapFormItem(u.editor.form.GetFormItem(0)).(*tview.InputField)
	u.app.SetFocus(field)
	bar := &functionBar{ui: u}
	labels := bar.labels()
	if labels[2] != "마우스 사용" || labels[3] != "마우스 해제" {
		t.Fatal("unavailable clipboard advertised", labels)
	}
	_, page := u.pages.GetFrontPage()
	page.PasteHandler()("123", func(p tview.Primitive) { u.app.SetFocus(p) })
	if field.GetText() != "123" {
		t.Fatal("terminal paste lost", field.GetText())
	}
}

func clickClipboardButton(u *UI, index int) {
	target := u.modeFocus()
	root := u.screenRoot().(*appScreen)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		panic(err)
	}
	defer screen.Fini()
	screen.SetSize(140, 40)
	root.SetRect(0, 0, 140, 40)
	root.Draw(screen)
	label := []string{"복사", "붙여넣기"}[index]
	for _, c := range root.controls {
		if c.target != target || c.button.GetLabel() != label {
			continue
		}
		x, y, _, _ := c.button.GetRect()
		event := tcell.NewEventMouse(x+1, y, tcell.Button1, 0)
		focus := func(p tview.Primitive) { u.app.SetFocus(p) }
		root.MouseHandler()(tview.MouseLeftDown, event, focus)
		root.MouseHandler()(tview.MouseLeftClick, event, focus)
		return
	}
	panic("input clipboard button missing")
}

func TestNoClipboardButtonsWithoutTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	u := editorFixture(t)
	if u.hasClipboard() {
		t.Fatal("clipboard unexpectedly available")
	}
	root := u.screenRoot().(*appScreen)
	if root.GetItemCount() != 2 {
		t.Fatal("clipboard toolbar visible")
	}
	check := func() {
		t.Helper()
		_, page := u.pages.GetFrontPage()
		visitWidgets(page, func(p tview.Primitive) {
			if b, ok := p.(*tview.Button); ok && (strings.Contains(b.GetLabel(), "복사") || strings.Contains(b.GetLabel(), "붙여넣기")) {
				t.Fatal("clipboard button visible", b.GetLabel())
			}
		})
	}
	check()
	u.calculator()
	check()
	u.bookmarks()
	check()
	_, before := u.pages.GetFrontPage()
	u.clipboardMenu()
	if _, after := u.pages.GetFrontPage(); before != after {
		t.Fatal("unsupported clipboard menu opened")
	}
}
