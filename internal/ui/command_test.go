package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func editorFixture(t *testing.T) *UI {
	t.Helper()
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages(), dir: t.TempDir()}
	u.build()
	u.selected = &garden.Command{Path: []string{"query", "block"}, Usage: "demo query block <height> [label] [flags]", Help: "block help", Flags: []garden.Flag{{Name: "chain-id", Type: "string"}, {Name: "output", Type: "string", Description: "format (text|json)"}, {Name: "trace", Type: "bool"}}}
	u.descriptor = &garden.Descriptor{Binary: "/bin/echo", Home: "/tmp/chain-a", Root: u.selected}
	u.commandForm()
	return u
}
func TestCommandFormValidationPersistence(t *testing.T) {
	u := editorFixture(t)
	ed := u.editor
	u.submitForm(false)
	if !strings.Contains(ed.feedback.GetText(true), "height") {
		t.Fatal("missing inline validation")
	}
	if name, _ := u.pages.GetFrontPage(); name != "command" {
		t.Fatal("validation lost form")
	}
	height := ed.form.GetFormItem(0).(*tview.InputField)
	height.SetText("42")
	// Required height, optional label checkbox/input, chain-id checkbox/input.
	ed.form.GetFormItem(2).(*fieldRow).check.SetChecked(true)
	ed.form.GetFormItem(2).(*fieldRow).FormItem.(*tview.InputField).SetText("chain-a")
	if e := u.saveCommon(); e != nil {
		t.Fatal(e)
	}
	var prefs garden.Preferences
	if e := garden.Load(filepath.Join(u.dir, "common-flags.json"), &prefs); e != nil {
		t.Fatal(e)
	}
	if prefs[u.preferenceKey()]["chain-id"].Value != "chain-a" {
		t.Fatal(prefs)
	}
	u.leaveCommand()
	u.commandForm()
	if u.editor.values["arg:0"].Value != "42" {
		t.Fatal("command draft lost")
	}
	if got := u.editor.values["flag:chain-id"]; !got.Enabled || got.Value != "chain-a" {
		t.Fatal(got)
	}
	u.leaveCommand()
	u.descriptor.Home = "/tmp/chain-b"
	u.commandForm()
	if u.editor.values["flag:chain-id"].Enabled {
		t.Fatal("common values leaked across homes")
	}
}
func TestCommandFormScreenAndInsert(t *testing.T) {
	u := editorFixture(t)
	s := tcell.NewSimulationScreen("UTF-8")
	if e := s.Init(); e != nil {
		t.Fatal(e)
	}
	defer s.Fini()
	s.SetSize(120, 40)
	u.pages.SetRect(0, 0, 120, 40)
	u.pages.Draw(s)
	s.Show()
	cells, w, h := s.GetContents()
	var text strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			text.WriteString(string(cells[y*w+x].Runes))
		}
	}
	if !strings.Contains(text.String(), "*   height") || !strings.Contains(text.String(), "--chain-id") {
		t.Fatal("form fields not drawn")
	}
	input := u.editor.form.GetFormItem(0).(*tview.InputField)
	u.app.SetFocus(input)
	if !u.appendArg("1000000000000000001") || u.editor.values["arg:0"].Value != "1000000000000000001" {
		t.Fatal("calculator/bookmark value not connected to argv")
	}
	// Narrow windows must not let the focused input overwrite the help panel.
	s.SetSize(80, 24)
	u.pages.SetRect(0, 0, 80, 24)
	u.pages.Draw(s)
	s.Show()
	x, _, width, _ := u.editor.form.GetRect()
	_, y, _, _ := input.GetRect()
	border, _, _, _ := s.GetContent(x+width-1, y)
	if border != '║' && border != '│' && border != '█' && border != '░' {
		t.Fatalf("input crossed form border: %q", border)
	}
	// Return from an overlay to the same form and retain values.
	u.alert("test")
	u.close()
	if name, _ := u.pages.GetFrontPage(); name != "command" {
		t.Fatal(name)
	}
	if input.GetText() != "1000000000000000001" {
		t.Fatal("lost input")
	}
}
func TestTextResultQuery(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	u.raw = "plain text"
	u.resultQuery()
	if _, page := u.pages.GetFrontPage(); queryResultPane(page) == nil {
		t.Fatal("expected text query window")
	}
}

func TestFirstRunEventLoop(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages(), dir: t.TempDir()}
	u.build()
	u.app.SetScreen(screen).SetRoot(u.screenRoot(), true)
	u.installInput()
	u.installStartup()
	after := u.app.GetAfterDrawFunc()
	stopped := false
	u.app.SetAfterDrawFunc(func(s tcell.Screen) {
		after(s)
		name, _ := u.pages.GetFrontPage()
		if name == "dialog" && !stopped {
			stopped = true
			go u.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
		}
	})
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first launch UI deadlocked")
	}
}

func TestPinnedActionsAndTerminalPaste(t *testing.T) {
	u := editorFixture(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	u.pages.SetRect(0, 0, 80, 24)
	u.pages.Draw(screen)
	// Focus a later field and ensure actions stay on the final three rows.
	u.editor.form.SetFocus(u.editor.form.GetFormItemCount() - 1)
	u.app.SetFocus(u.editor.form)
	u.pages.Draw(screen)
	screen.Show()
	cells, w, h := screen.GetContents()
	var bottom strings.Builder
	for _, c := range cells[(h-3)*w:] {
		bottom.WriteString(string(c.Runes))
	}
	text := strings.ReplaceAll(bottom.String(), " ", "")
	for _, label := range []string{"검증후실행", "대화형실행", "돌아가기"} {
		if !strings.Contains(text, label) {
			t.Fatalf("missing pinned action %s: %s", label, text)
		}
	}
	input := u.editor.form.GetFormItem(0).(*tview.InputField)
	input.SetText("1")
	complete := func(string) []string { return []string{"123"} }
	u.editor.autocomplete[input] = complete
	input.SetAutocompleteFunc(complete)
	u.app.SetFocus(input)
	_, page := u.pages.GetFrontPage()
	page.PasteHandler()("2\n", func(p tview.Primitive) { u.app.SetFocus(p) })
	if input.GetText() != "12" {
		t.Fatal("paste blocked by completion", input.GetText())
	}
	page.PasteHandler()("bad", func(tview.Primitive) {})
	if input.GetText() != "12" {
		t.Fatal("numeric validation bypassed")
	}
}

func TestSearchEnterFirstResult(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	u.populate("")
	u.app.SetFocus(u.search)
	u.search.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	if name, _ := u.pages.GetFrontPage(); name != "command" {
		t.Fatal("search Enter did not open form")
	}
	u.leaveCommand()
	u.search.SetText("no such command")
	u.search.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	if name, _ := u.pages.GetFrontPage(); name != "main" {
		t.Fatal("empty result opened form")
	}
}

func TestInlineFieldsPreviewAndDescription(t *testing.T) {
	file := mockClipboard(t)
	u := editorFixture(t)
	ed := u.editor
	ed.command.Help = "full help should not appear in field detail"
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	root := u.screenRoot()
	root.SetRect(0, 0, 100, 30)
	root.Draw(screen)
	row := ed.form.GetFormItem(2).(*fieldRow)
	_, cy, _, _ := row.check.GetRect()
	_, iy, _, _ := row.FormItem.GetRect()
	if cy != iy {
		t.Fatal("checkbox not inline")
	}
	row.check.SetChecked(true)
	input := row.FormItem.(*tview.InputField)
	input.SetText("chain with spaces")
	u.app.SetFocus(input)
	if ed.active != input {
		t.Fatal("active input not tracked")
	}
	if strings.Contains(ed.detail.GetText(false), ed.command.Help) {
		t.Fatal("full help leaked into detail")
	}
	preview := ed.preview.GetText(false)
	if !strings.Contains(preview, "--chain-id=chain with spaces") {
		t.Fatal(preview)
	}
	_, front := u.pages.GetFrontPage()
	copyButton := front.(*commandPage).GetItem(1).(*tview.Flex).GetItem(0).(*tview.Flex).GetItem(0).(*tview.Button)
	copyButton.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	b, err := os.ReadFile(file)
	if err != nil || string(b) != preview {
		t.Fatal("command copy mismatch", string(b), err)
	}
	row.check.SetChecked(false)
	if strings.Contains(ed.preview.GetText(false), "--chain-id") {
		t.Fatal("disabled flag included")
	}
	// Keyboard visits the checkbox and its enabled input before the next row.
	row.check.SetChecked(true)
	ed.form.SetFocus(2)
	u.app.SetFocus(ed.form)
	if u.app.GetFocus() != row.check {
		t.Fatal("checkbox focus missing")
	}
	row.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	if u.app.GetFocus() != input {
		t.Fatal("Tab skipped enabled input")
	}
	root.Draw(screen)
	screen.Show()
	cells, w, h := screen.GetContents()
	var bottom strings.Builder
	for _, c := range cells[(h-1)*w:] {
		bottom.WriteString(string(c.Runes))
	}
	if !strings.Contains(bottom.String(), "F1") || !strings.Contains(bottom.String(), "F4") {
		t.Fatal(bottom.String())
	}
}

func TestCheckboxNavigationAndFormLayout(t *testing.T) {
	u := editorFixture(t)
	u.installInput()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 24)
	root := u.screenRoot()
	root.SetRect(0, 0, 100, 24)
	root.Draw(s)
	ed := u.editor
	row := ed.form.GetFormItem(1).(*fieldRow)
	ed.form.SetFocus(1)
	u.app.SetFocus(ed.form)
	ed.form.InputHandler()(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	if u.app.GetFocus() != row.FormItem {
		t.Fatalf("checking focus=%T target=%p actual=%p checked=%v values=%v", u.app.GetFocus(), row.FormItem, u.app.GetFocus(), row.check.IsChecked(), ed.values)
	}
	root.Draw(s)
	x, y, _, _ := row.check.GetRect()
	got, _, _, _ := s.GetContent(x, y)
	if got != 'O' {
		t.Fatalf("checked marker %q", got)
	}
	ed.form.SetFocus(1)
	u.app.SetFocus(ed.form)
	ed.form.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	next := ed.form.GetFormItem(2).(*fieldRow)
	if u.app.GetFocus() != next.check {
		t.Fatal("down did not reach next checkbox")
	}
	ed.form.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	if u.app.GetFocus() != row.check {
		t.Fatal("up did not reach previous checkbox")
	}
	root.Draw(s)
	x, y, _, _ = next.check.GetRect()
	for _, a := range []tview.MouseAction{tview.MouseLeftDown, tview.MouseLeftClick} {
		ed.form.MouseHandler()(a, tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	}
	if u.app.GetFocus() != next.FormItem {
		t.Fatal("mouse check focused wrong field")
	}
	for _, want := range []tview.Primitive{next.check, row.FormItem, row.check, ed.form.GetFormItem(0)} {
		ed.form.InputHandler()(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
		if u.app.GetFocus() != want {
			t.Fatalf("reverse form focus got %p want %p", u.app.GetFocus(), want)
		}
	}
	_, p := u.pages.GetFrontPage()
	page := p.(*commandPage)
	if !strings.HasPrefix(page.GetItem(0).(*tview.TextView).GetText(false), "사용법: ") {
		t.Fatal("usage label missing")
	}
	buttons := page.GetItem(page.GetItemCount() - 1).(*tview.Flex)
	count := 0
	root.Draw(s)
	for i := 0; i < buttons.GetItemCount(); i++ {
		b, ok := buttons.GetItem(i).(*edgeButton)
		if !ok {
			continue
		}
		count++
		bx, by, bw, _ := b.GetRect()
		left, _, _, _ := s.GetContent(bx, by)
		right, _, _, _ := s.GetContent(bx+bw-1, by)
		if left != '[' || right != ']' {
			t.Fatal("button brackets not at edges")
		}
	}
	if count != 3 {
		t.Fatal("unexpected button count", count)
	}
	// The viewport is smaller than the form; moving to its last row moves the thumb.
	s.SetSize(100, 20)
	root.SetRect(0, 0, 100, 20)
	pane := page.GetItem(2).(*fieldPane)
	ed.form.SetFocus(0)
	u.app.SetFocus(ed.form)
	root.Draw(s)
	px, py, pw, ph := pane.GetRect()
	first, _, _, _ := s.GetContent(px+pw-1, py+1)
	ed.form.SetFocus(ed.form.GetFormItemCount() - 1)
	u.app.SetFocus(ed.form)
	root.Draw(s)
	last, _, _, _ := s.GetContent(px+pw-1, py+ph-2)
	topAfter, _, _, _ := s.GetContent(px+pw-1, py+1)
	if first != '█' || last != '█' || topAfter != '░' {
		t.Fatal("scrollbar endpoints", first, last)
	}
}

func TestCLIAndAddressBookFunctionKeys(t *testing.T) {
	u := editorFixture(t)
	u.installInput()
	u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if u.pickerList == nil || !u.pickerList.HasFocus() {
		t.Fatal("F2 did not open address book")
	}
	u.close()
	u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone))
	_, page := u.pages.GetFrontPage()
	form := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Form)
	found := false
	for i := 0; i < form.GetButtonCount(); i++ {
		if strings.Contains(form.GetButton(i).GetLabel(), "재분석") {
			found = true
		}
	}
	if !found {
		t.Fatal("CLI settings missing reanalysis action")
	}
}

func TestCommandPageClearsUnderlyingGaps(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	// A dense underlying page exposes transparent gaps without relying on a particular command list.
	background := tview.NewTextView().SetText(strings.Repeat(strings.Repeat("Z", 100)+"\n", 30))
	u.pages.AddPage("main", background, true, true)
	u.commandForm()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 30)
	u.pages.SetRect(0, 0, 100, 30)
	u.pages.Draw(s)
	_, p := u.pages.GetFrontPage()
	page := p.(*commandPage)
	preview := page.GetItem(1).(*tview.Flex)
	x, y, _, _ := preview.GetRect()
	// The space below the copy button used to reveal old command text.
	for dx := 0; dx < 6; dx++ {
		r, _, _, _ := s.GetContent(x+dx, y+1)
		if r != ' ' {
			t.Fatalf("preview gap shows underlying content: %q", r)
		}
	}
	buttons := page.GetItem(page.GetItemCount() - 1).(*tview.Flex)
	first := buttons.GetItem(0).(*edgeButton)
	bx, by, bw, _ := first.GetRect()
	r, _, _, _ := s.GetContent(bx+bw, by)
	if r != ' ' {
		t.Fatalf("button gap shows underlying content: %q", r)
	}
}

func TestCommandPageTabIncludesButtons(t *testing.T) {
	u := editorFixture(t)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 24)
	root := u.screenRoot()
	root.SetRect(0, 0, 100, 24)
	root.Draw(s)
	_, p := u.pages.GetFrontPage()
	page := p.(*commandPage)
	copyButton := page.GetItem(1).(*tview.Flex).GetItem(0).(*tview.Flex).GetItem(0).(*tview.Button)
	buttons := page.GetItem(page.GetItemCount() - 1).(*tview.Flex)
	first := u.editor.form.GetFormItem(0)
	label := u.editor.form.GetFormItem(1).(*fieldRow)
	chain := u.editor.form.GetFormItem(2).(*fieldRow)
	output := u.editor.form.GetFormItem(3).(*fieldRow)
	trace := u.editor.form.GetFormItem(4).(*fieldRow)
	label.check.SetChecked(true)
	output.check.SetChecked(true)
	order := []tview.Primitive{copyButton, first, label.check, label.FormItem, chain.check, output.check, output.FormItem, trace.check, buttons.GetItem(0).(*edgeButton).Button, buttons.GetItem(2).(*edgeButton).Button, buttons.GetItem(4).(*edgeButton).Button}
	for _, key := range []tcell.Key{tcell.KeyTab, tcell.KeyBacktab} {
		u.app.SetFocus(copyButton)
		for step := 1; step <= len(order); step++ {
			root.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
			root.Draw(s)
			index := step % len(order)
			if key == tcell.KeyBacktab {
				index = (len(order) - step) % len(order)
			}
			if u.app.GetFocus() != order[index] {
				t.Fatalf("key %v step %d: got %p want %p", key, step, u.app.GetFocus(), order[index])
			}
		}
	}
	// Changes to enabled fields immediately alter the cycle.
	label.check.SetChecked(false)
	u.app.SetFocus(label.check)
	root.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), func(p tview.Primitive) { u.app.SetFocus(p) })
	if u.app.GetFocus() != chain.check {
		t.Fatal("disabled input still reachable")
	}
}
