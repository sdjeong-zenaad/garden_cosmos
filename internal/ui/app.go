package ui

import (
	"context"
	"os"
	"path/filepath"

	"garden_cosmos/internal/garden"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	focusRepairFrom      *tview.Button
	focusRepairTarget    tview.Primitive
	focusRepairQueued    bool
	focusOrders          map[tview.Primitive]func() []tview.Primitive
	calculatorSave       *deferredSave
	calculatorCancel     context.CancelFunc
	launchTask           func(func())
	postTask             func(func())
	screen               *appScreen
	calculatorPage       tview.Primitive
	terminalMode         bool
	session              *garden.Session
	displayOutput        string
	pickerList           *tview.List
	pickerValue          func() (string, bool)
	editor               *commandEditor
	drafts               map[string]map[string]garden.FieldValue
	app                  *tview.Application
	pages                *tview.Pages
	list                 *tview.List
	help, output, status *tview.TextView
	search               *tview.InputField
	dir                  string
	profile              garden.Profile
	descriptor           *garden.Descriptor
	store                garden.Store
	selected             *garden.Command
	visibleCommands      []*garden.Command
	raw, rawErr          string
	busy                 bool
	quitRequested        bool
	cancel               context.CancelFunc
}

func Run(dir string, session *garden.Session, mouse bool) error {
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages(), dir: dir, session: session}
	if e := garden.Load(filepath.Join(dir, "profile.json"), &u.profile); e != nil && !os.IsNotExist(e) {
		return e
	}
	if e := garden.Load(filepath.Join(dir, "bookmarks.json"), &u.store); e != nil && !os.IsNotExist(e) {
		return e
	}
	u.launchTask = func(f func()) { go f() }
	u.postTask = func(f func()) { u.app.QueueUpdateDraw(f) }
	u.build()
	u.terminalMode = !mouse
	u.app.SetRoot(u.screenRoot(), true).EnableMouse(mouse).EnablePaste(true)
	u.installInput()
	u.installStartup()
	err := u.app.Run()
	if u.calculatorCancel != nil {
		u.calculatorCancel()
	}
	saveErr := u.flushCalculator()
	if err != nil {
		return err
	}
	return saveErr
}
func (u *UI) installStartup() {
	first := true
	u.app.SetAfterDrawFunc(func(s tcell.Screen) {
		if !first {
			return
		}
		first = false
		// tview holds its draw lock here. Focus changes must run in the event loop.
		go u.app.QueueUpdateDraw(func() {
			if u.profile.Binary == "" {
				u.settings()
			} else {
				u.refresh(false)
			}
		})
	})
}

func (u *UI) build() {
	u.status = tview.NewTextView().SetDynamicColors(true)
	u.search = tview.NewInputField().SetLabel("명령 검색 / ")
	u.list = tview.NewList().ShowSecondaryText(false)
	u.list.SetBorder(true).SetTitle(" COMMANDS ")
	u.help = tview.NewTextView().SetScrollable(true).SetWrap(true)
	u.help.SetBorder(true).SetTitle(" HELP · 사용법 / 플래그 ")
	u.output = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(false)
	u.output.SetBorder(true).SetTitle(" RESULT · JSON: F9 jq · 방향키 / PgUp / PgDn ")
	u.output.SetText("")
	u.app.SetMouseCapture(u.contextMouse)
	u.search.SetChangedFunc(func(s string) { u.populate(s) })
	u.search.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEnter {
			if u.list.GetItemCount() > 0 {
				u.list.SetCurrentItem(0)
				u.selectIndex(0)
				u.commandForm()
			}
			return nil
		}
		if e.Key() == tcell.KeyUp || e.Key() == tcell.KeyDown {
			if e.Key() == tcell.KeyUp && u.list.GetItemCount() > 0 {
				u.list.SetCurrentItem(u.list.GetItemCount() - 1)
			}
			u.app.SetFocus(u.list)
			return nil
		}
		return e
	})
	u.list.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyRune && e.Rune() >= ' ' && e.Modifiers() & ^tcell.ModShift == 0 {
			u.app.SetFocus(u.search)
			u.search.InputHandler()(e, func(p tview.Primitive) { u.app.SetFocus(p) })
			return nil
		}
		return e
	})
	u.list.SetChangedFunc(func(i int, main, secondary string, r rune) { u.selectIndex(i) })
	u.list.SetSelectedFunc(func(i int, _, _ string, _ rune) {
		// Mouse activation precedes the list's changed callback.
		u.selectIndex(i)
		u.commandForm()
	})
	left := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.search, 1, 0, true).AddItem(u.list, 0, 1, false)
	right := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.help, 0, 1, false).AddItem(u.output, 0, 2, false)
	body := tview.NewFlex().AddItem(left, 0, 1, true).AddItem(right, 0, 2, false)
	root := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.status, 2, 0, false).AddItem(body, 0, 1, true)
	u.setFocusOrder(root, func() []tview.Primitive { return []tview.Primitive{u.search, u.list, u.output} })
	u.pages.AddPage("main", root, true, true)
	u.message("gcosm · CLI를 선택하세요 (F1)")
}
func (u *UI) message(s string) { u.status.SetText(tview.Escape(garden.Clean(s))) }
func (u *UI) close() {
	if err := u.flushCalculator(); err != nil {
		u.message(err.Error())
		return
	}
	if u.calculatorCancel != nil {
		u.calculatorCancel()
		u.calculatorCancel = nil
	}
	u.calculatorPage = nil
	u.forgetPageFocus("dialog")
	u.pages.RemovePage("dialog")
	if u.editor != nil {
		u.app.SetFocus(u.editor.form)
	} else {
		u.app.SetFocus(u.list)
	}
}
func (u *UI) modal(p tview.Primitive, w, h int) {
	if u.calculatorCancel != nil {
		u.calculatorCancel()
		u.calculatorCancel = nil
	}
	if err := u.flushCalculator(); err != nil {
		u.message(err.Error())
	}
	u.calculatorPage = nil
	_, _, sw, sh := u.pages.GetRect()
	if sw > 0 && w > sw {
		w = sw
	}
	if sh > 0 && h > sh {
		h = sh
	}
	box := tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(p, w, 1, true).AddItem(nil, 0, 1, false)
	outer := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(nil, 0, 1, false).AddItem(box, h, 1, true).AddItem(nil, 0, 1, false)
	u.forgetPageFocus("dialog")
	u.pages.AddPage("dialog", outer, true, true)
	u.app.SetFocus(p)
}
func (u *UI) alert(s string) {
	m := newActionModal().SetText(tview.Escape(s)).AddButtons([]string{"확인"}).SetDoneFunc(func(int, string) { u.close() })
	u.modal(m, 76, 14)
}
func (u *UI) flags() { u.commandForm() }
func (u *UI) appendArg(s string) bool {
	if u.editor == nil || u.editor.active == nil || !u.editor.values[u.editor.activeKey].Enabled {
		u.message("명령 폼에서 사용할 입력 칸을 활성화하고 선택하세요.")
		return false
	}
	u.editor.active.SetText(s)
	return true
}
func (u *UI) confirm(interactive bool) { u.commandForm() }
