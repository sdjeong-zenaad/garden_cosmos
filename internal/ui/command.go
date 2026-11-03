package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type commandEditor struct {
	pendingCompletion *tview.InputField
	autocomplete      map[*tview.InputField]func(string) []string
	recorded          map[string]garden.FieldValue
	activeKey         string
	command           *garden.Command
	fields            []garden.Field
	values            map[string]garden.FieldValue
	form              *tview.Form
	feedback          *tview.TextView
	active            *tview.InputField
	preview           *tview.TextView
	detail            *tview.TextView
}

func (u *UI) preferenceKey() string { return u.descriptor.Binary + "\n" + u.descriptor.Home }
func (u *UI) commandForm() {
	if u.busy || u.selected == nil || u.descriptor == nil {
		return
	}
	if len(u.selected.Children) > 0 {
		u.message("하위 실행 명령을 선택하세요.")
		return
	}
	if u.selected.Usage == "" {
		u.alert("도움말을 읽지 못한 명령입니다. F5로 재분석하세요.")
		return
	}
	var prefs garden.Preferences
	if e := garden.Load(filepath.Join(u.dir, "common-flags.json"), &prefs); e != nil && !os.IsNotExist(e) {
		u.alert(e.Error())
		return
	}
	ed := &commandEditor{command: u.selected, fields: garden.Fields(u.selected), values: map[string]garden.FieldValue{}, form: tview.NewForm(), feedback: tview.NewTextView().SetDynamicColors(true)}
	ed.autocomplete = map[*tview.InputField]func(string) []string{}
	u.editor = ed
	detail := tview.NewTextView().SetWrap(true)
	ed.detail = detail
	ed.preview = tview.NewTextView().SetWrap(true)
	copyCommand := func() {
		if err := u.copyClipboard(ed.preview.GetText(false)); err != nil {
			ed.feedback.SetText(tview.Escape(err.Error()))
		} else {
			ed.feedback.SetText("명령줄 복사 완료")
		}
	}
	ed.preview.SetMouseCapture(func(a tview.MouseAction, e *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if a == tview.MouseLeftClick {
			copyCommand()
			return a, nil
		}
		return a, e
	})
	for index, field := range ed.fields {
		f := field
		v := garden.FieldValue{Enabled: f.Required, Value: f.Default}
		if f.Flag && f.Name == "home" && u.descriptor.Home != "" {
			v.Value = u.descriptor.Home
			v.Enabled = true
		}
		if draft, ok := u.drafts[u.preferenceKey()+"\n"+strings.Join(ed.command.Path, " ")][f.Key]; ok {
			v = draft
		}
		if f.Flag && garden.CommonFlag(f.Name) {
			if saved, ok := prefs[u.preferenceKey()][f.Name]; ok && garden.ValidateField(f, saved.Value) == nil {
				v = saved
			}
		}
		if f.Required {
			v.Enabled = true
		}
		ed.values[f.Key] = v
		label := f.Name
		if f.Flag {
			label = "--" + label
		}
		if f.Required {
			label = "*   " + label
		}
		showDetail := func() { ed.activeKey = f.Key; ed.active = nil; detail.SetText(f.Description) }
		setValue := func(s string) {
			current := ed.values[f.Key]
			current.Value = s
			ed.values[f.Key] = current
			u.updateCommandPreview()
		}
		var item tview.FormItem
		if len(f.Choices) > 0 {
			options := append([]string{}, f.Choices...)
			idx := 0
			// Stale drafts must never extend this command's allowed choices.
			if garden.ValidateField(f, v.Value) != nil {
				v.Value = f.Default
			}
			for i, c := range options {
				if c == v.Value {
					idx = i
				}
			}

			drop := tview.NewDropDown().SetLabel(label).SetOptions(options, func(s string, _ int) { setValue(s) }).SetCurrentOption(idx)
			drop.SetDisabled(!v.Enabled)
			drop.SetFocusFunc(showDetail)
			item = drop
		} else {
			input := tview.NewInputField().SetLabel(label).SetText(v.Value).SetFieldWidth(0).SetChangedFunc(setValue)
			input.SetDisabled(!v.Enabled)
			if strings.HasPrefix(f.Type, "int") || strings.HasPrefix(f.Type, "uint") {
				input.SetAcceptanceFunc(func(text string, _ rune) bool {
					if text == "" || text == "-" {
						return true
					}
					for i, r := range text {
						if i == 0 && r == '-' {
							continue
						}
						if r < '0' || r > '9' {
							return false
						}
					}
					return true
				})
			}

			styleAutocomplete(input)
			var completionValues []string
			ed.autocomplete[input] = func(s string) []string {
				completionValues = nil
				var out []string
				for _, e := range u.store.Suggestions(f, s) {
					label := e.Value
					if e.Nickname != "" {
						label += " · " + strings.Join(strings.Fields(e.Nickname), " ")
					}
					out = append(out, label)
					completionValues = append(completionValues, e.Value)
				}
				if f.Flag && (f.Name == "home" || strings.Contains(f.Name, "file") || strings.Contains(f.Name, "dir")) {
					matches, _ := filepath.Glob(s + "*")
					if len(matches) > 30 {
						matches = matches[:30]
					}
					out = append(out, matches...)
					completionValues = append(completionValues, matches...)
				}
				return out
			}
			input.SetAutocompleteFunc(ed.autocomplete[input])
			completedByMouse := false
			input.SetAutocompletedFunc(func(text string, index int, source int) bool {
				if source == tview.AutocompletedNavigate {
					return false
				}
				if index < 0 || index >= len(completionValues) {
					return true
				}
				text = completionValues[index]
				completedByMouse = source == tview.AutocompletedClick
				input.SetText(text)
				u.store.Touch(text)
				if err := u.rememberInputs(); err != nil {
					ed.feedback.SetText(tview.Escape(err.Error()))
				}
				if err := u.persistEntries(); err != nil {
					ed.feedback.SetText(tview.Escape(err.Error()))
				}
				return true
			})
			input.SetBlurFunc(func() {
				if err := u.rememberInputs(); err != nil {
					ed.feedback.SetText(tview.Escape(err.Error()))
				}
			})
			input.SetFocusFunc(func() {
				showDetail()
				ed.active = input
				if completedByMouse {
					completedByMouse = false
					return
				}
				ed.pendingCompletion = input
			})
			item = input
		}
		if !f.Required {
			widget := item
			item = newFieldRow(item, v.Enabled, func(on bool) {
				current := ed.values[f.Key]
				current.Enabled = on
				ed.values[f.Key] = current
				widget.SetDisabled(!on)
				u.updateCommandPreview()
			}, showDetail, func(p tview.Primitive) { u.app.SetFocus(p) })
			item.(*fieldRow).check.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
				if e.Key() == tcell.KeyUp || e.Key() == tcell.KeyDown {
					for j := 0; j < ed.form.GetFormItemCount(); j++ {
						if row, ok := ed.form.GetFormItem(j).(*fieldRow); ok {
							row.backward = false
						}
					}
					next := index + 1
					if e.Key() == tcell.KeyUp {
						next = index - 1
					}
					next = max(0, min(len(ed.fields)-1, next))
					ed.form.SetFocus(next)
					u.app.SetFocus(ed.form)
					return nil
				}
				return e
			})
		}
		ed.form.AddFormItem(item)
	}
	ed.form.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyTab || e.Key() == tcell.KeyBacktab {
			for i := 0; i < ed.form.GetFormItemCount(); i++ {
				if row, ok := ed.form.GetFormItem(i).(*fieldRow); ok {
					row.backward = e.Key() == tcell.KeyBacktab
				}
			}
		}
		return e
	})
	ed.form.SetBorder(true)
	u.updateCommandPreview()
	buttons := tview.NewFlex()
	var actions []*tview.Button
	for i, b := range []struct {
		label string
		fn    func()
	}{
		{"검증 후 실행", func() { u.submitForm(false) }},
		{"대화형 실행", func() { u.submitForm(true) }},
		{"돌아가기", u.leaveCommand},
	} {
		button := tview.NewButton(b.label).SetSelectedFunc(b.fn)
		actions = append(actions, button)
		if i > 0 {
			buttons.AddItem(nil, 1, 0, false)
		}
		buttons.AddItem(&edgeButton{button}, 0, 1, i == 0)
	}
	usage := tview.NewTextView().SetText("사용법: " + ed.command.Usage)
	copyButton := tview.NewButton("[복사]").SetSelectedFunc(copyCommand)
	copyColumn := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(copyButton, 1, 0, true).AddItem(nil, 0, 1, false)
	previewRow := tview.NewFlex().AddItem(copyColumn, 8, 0, false).AddItem(nil, 1, 0, false).AddItem(ed.preview, 0, 1, false)
	pane := &fieldPane{Form: ed.form, focus: func(i int) { ed.form.SetFocus(i); u.app.SetFocus(ed.form) }}
	page := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(usage, 2, 0, false).
		AddItem(previewRow, 3, 0, false).
		AddItem(pane, 0, 1, true).
		AddItem(detail, 3, 0, false).
		AddItem(ed.feedback, 1, 0, false).
		AddItem(buttons, 1, 0, false)
	page.Box = tview.NewBox() // Clear gaps before drawing over the command browser.
	page.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch e.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			// One page-wide cycle includes buttons outside tview.Form. Rebuild it
			// after each checkbox change so disabled inputs are never focused.
			order := []tview.Primitive{copyButton}
			for i := 0; i < ed.form.GetFormItemCount(); i++ {
				item := ed.form.GetFormItem(i)
				if row, ok := item.(*fieldRow); ok {
					order = append(order, row.check)
					if row.check.IsChecked() {
						order = append(order, row.FormItem)
					}
				} else {
					order = append(order, item)
				}
			}
			for _, button := range actions {
				order = append(order, button)
			}
			current := -1
			for i, item := range order {
				if item == u.app.GetFocus() {
					current = i
					break
				}
			}
			next := (current + 1) % len(order)
			if e.Key() == tcell.KeyBacktab {
				next = (current - 1 + len(order)) % len(order)
				if current < 0 {
					next = len(order) - 1
				}
			}
			u.app.SetFocus(order[next])
			return nil
		case tcell.KeyF6:
			if ed.form.HasFocus() {
				u.app.SetFocus(actions[0])
			} else {
				u.app.SetFocus(ed.form)
			}
			return nil
		case tcell.KeyF7:
			u.submitForm(false)
			return nil
		case tcell.KeyF8:
			u.submitForm(true)
			return nil
		case tcell.KeyEscape:
			u.leaveCommand()
			return nil
		}
		return e
	})
	ed.recorded = map[string]garden.FieldValue{}
	for k, v := range ed.values {
		ed.recorded[k] = v
	}
	u.pages.AddPage("command", &commandPage{Flex: page, ui: u}, true, true)
	u.app.SetFocus(ed.form)
}
func (u *UI) leaveCommand() {
	if e := u.rememberInputs(); e != nil {
		u.alert(e.Error())
		return
	}
	if u.editor != nil {
		if u.drafts == nil {
			u.drafts = map[string]map[string]garden.FieldValue{}
		}
		values := map[string]garden.FieldValue{}
		for k, v := range u.editor.values {
			values[k] = v
		}
		u.drafts[u.preferenceKey()+"\n"+strings.Join(u.editor.command.Path, " ")] = values
	}
	u.pages.RemovePage("command")
	u.editor = nil
	u.app.SetFocus(u.list)
}
func (u *UI) saveCommon() error {
	if u.editor == nil {
		return nil
	}
	prefs := garden.Preferences{}
	if e := garden.Load(filepath.Join(u.dir, "common-flags.json"), &prefs); e != nil && !os.IsNotExist(e) {
		return e
	}
	if prefs == nil {
		prefs = garden.Preferences{}
	}
	key := u.preferenceKey()
	if prefs[key] == nil {
		prefs[key] = map[string]garden.FieldValue{}
	}
	for _, f := range u.editor.fields {
		if f.Flag && garden.CommonFlag(f.Name) {
			v := u.editor.values[f.Key]
			if v.Enabled {
				if e := garden.ValidateField(f, v.Value); e != nil {
					return fmt.Errorf("%s: %w", f.Name, e)
				}
			}
			prefs[key][f.Name] = v
		}
	}
	return garden.Save(filepath.Join(u.dir, "common-flags.json"), prefs)
}
func (u *UI) submitForm(interactive bool) {
	if u.editor == nil || u.busy {
		return
	}
	ed := u.editor
	args, e := garden.FormArgs(ed.command, ed.fields, ed.values)
	if e != nil {
		ed.feedback.SetText("[red]" + tview.Escape(e.Error()))
		return
	}

	if e := u.rememberInputs(); e != nil {
		ed.feedback.SetText(tview.Escape(e.Error()))
		return
	}
	u.confirmArgs(interactive, args, ed.command.Path)
}

// Terminal bracketed paste follows the same validation as the clipboard menu.
type commandPage struct {
	*tview.Flex
	ui *UI
}

func (p *commandPage) PasteHandler() func(string, func(tview.Primitive)) {
	return func(text string, focus func(tview.Primitive)) {
		u := p.ui
		field := focusedInput(p)
		if field == nil || u.editor == nil {
			return
		}
		text, err := garden.ClipboardFieldText(text)
		if err != nil {
			u.editor.feedback.SetText(tview.Escape(err.Error()))
			return
		}
		if complete := u.editor.autocomplete[field]; complete != nil {
			field.SetAutocompleteFunc(func(string) []string { return nil })
			defer field.SetAutocompleteFunc(complete)
		}
		field.PasteHandler()(text, focus)
	}
}

func (u *UI) updateCommandPreview() {
	ed := u.editor
	if ed == nil || ed.preview == nil {
		return
	}
	args := garden.CommandArgs(ed.command, ed.fields, ed.values)
	binary := u.descriptor.Binary
	if found, err := exec.LookPath(filepath.Base(binary)); err == nil && found == binary {
		binary = filepath.Base(binary)
	}
	ed.preview.SetText(garden.Preview(binary, args))
}
