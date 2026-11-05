package ui

import (
	"github.com/gdamore/tcell/v2"
)

func (u *UI) installInput() {
	u.app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		u.repairFocus()
		if u.cycleFocus(nil, e.Key()) {
			return nil
		}
		if u.screen != nil && u.screen.localControlInput(e) {
			return nil
		}
		if u.moveButtonFocus(e.Key()) {
			return nil
		}
		if e.Key() == tcell.KeyF3 || e.Key() == tcell.KeyF4 {
			u.setTerminalMode(e.Key() == tcell.KeyF4)
			return nil
		}
		if e.Key() == tcell.KeyF10 {
			if err := u.flushCalculator(); err != nil {
				u.alert(err.Error())
				return nil
			}
			if err := u.rememberInputs(); err != nil {
				u.alert(err.Error())
				return nil
			}
			if u.cancel != nil {
				u.quitRequested = true
				u.cancel()
				u.message("작업 취소 후 종료 중…")
			} else {
				u.app.Stop()
			}
			return nil
		}
		if e.Key() == tcell.KeyCtrlC {
			if u.cancel != nil {
				u.cancel()
				u.message("취소 요청: 트랜잭션이 이미 전송되었을 수 있습니다. 상태를 확인하세요.")
				return nil
			}
			if e := u.rememberInputs(); e != nil {
				u.alert(e.Error())
				return nil
			}
			if err := u.flushCalculator(); err != nil {
				u.alert(err.Error())
				return nil
			}
			u.app.Stop()
			return nil
		}
		if e.Key() == tcell.KeyF12 {
			name, page := u.pages.GetFrontPage()
			if name == "clipboard" {
				return nil
			}
			if page != nil && page == u.calculatorPage {
				u.close()
			} else {
				u.calculator()
			}
			return nil
		}
		name, _ := u.pages.GetFrontPage()
		if e.Key() == tcell.KeyF1 {
			u.settings()
			return nil
		}
		if name == "clipboard" {
			return e
		}
		if name == "command" {
			if e.Key() == tcell.KeyF2 {
				u.bookmarks()
				return nil
			}
			return e
		}
		if name != "main" {
			if e.Key() == tcell.KeyEscape {
				u.close()
				return nil
			}
			return e
		}
		switch e.Key() {
		case tcell.KeyF2:
			u.bookmarks()
			return nil
		case tcell.KeyF5:
			u.refresh(true)
			return nil
		case tcell.KeyF6:
			u.configs()
			return nil
		case tcell.KeyF7:
			u.confirm(false)
			return nil
		case tcell.KeyF8:
			u.confirm(true)
			return nil
		case tcell.KeyF9:
			u.resultQuery()
			return nil
		case tcell.KeyEscape:
			u.app.SetFocus(u.list)
			return nil

		}
		return e
	})
}
