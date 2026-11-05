package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Keep the complete text in the view; wrap long lines for readability.
type queryView struct{ *tview.TextView }

func (v *queryView) Draw(s tcell.Screen) {
	v.TextView.Draw(s)
	x, y, w, h := v.GetInnerRect()
	if w <= 0 || h <= 0 {
		return
	}
	total := max(1, v.GetWrappedLineCount())
	row, _ := v.GetScrollOffset()
	thumb := min(h, max(1, h*h/total))
	top := 0
	if total > h {
		top = min(h-thumb, max(0, row)*(h-thumb)/(total-h))
	}
	style := tcell.StyleDefault.Foreground(tcell.ColorTurquoise).Background(tview.Styles.PrimitiveBackgroundColor)
	for i := 0; i < h; i++ {
		ch := '░'
		if i >= top && i < top+thumb {
			ch = '█'
		}
		s.SetContent(x+w, y+i, ch, nil, style)
	}
}
func (v *queryView) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return v.WrapMouseHandler(func(a tview.MouseAction, e *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := e.Position()
		bx, by, w, h := v.GetInnerRect()
		if a == tview.MouseLeftClick && x == bx+w && y >= by && y < by+h {
			total := v.GetWrappedLineCount()
			v.ScrollTo(max(0, total-h)*(y-by)/max(1, h-1), 0)
			focus(v)
			return true, nil
		}
		return v.TextView.MouseHandler()(a, e, focus)
	})
}
func (u *UI) openResultQuery() {
	raw := u.querySource()
	isJSON := json.Valid([]byte(raw))
	title, label, initial := " 원본 텍스트 · 결과쿼리 ", "rg > ", ""
	filterOutput := garden.FilterText
	if isJSON {
		title, label, initial = " 원본 JSON · 결과쿼리 ", "jq > ", "."
		filterOutput = garden.FilterJSON
	}
	view := &queryView{tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)}
	view.SetBorder(true).SetTitle(title)
	colored := colorOutput(raw)
	view.SetText(colored)
	result := &queryView{tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)}
	result.SetBorder(true).SetTitle(" 쿼리 결과 ")
	result.SetText(colored)
	query := tview.NewInputField().SetLabel(label).SetText(initial)
	status := tview.NewTextView()
	page := tview.NewFlex().SetDirection(tview.FlexRow)
	page.Box = tview.NewBox()
	page.AddItem(view, 0, 1, false).AddItem(query, 1, 0, true).AddItem(status, 1, 0, false).AddItem(result, 0, 1, false)
	apply := func() {
		if u.busy {
			return
		}
		u.busy = true
		status.SetText("실행 중…")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		u.cancel = cancel
		filter := query.GetText()
		go func() {
			out, err := filterOutput(ctx, raw, filter)
			cancel()
			u.app.QueueUpdateDraw(func() {
				u.busy = false
				u.cancel = nil
				if u.quitRequested {
					u.app.Stop()
					return
				}
				// A closed/replaced dialog must not update another screen.
				_, front := u.pages.GetFrontPage()
				if front != page {
					return
				}
				if err != nil {
					status.SetText(err.Error())
					return
				}
				status.SetText("")
				if !isJSON && out == "" {
					status.SetText("검색 결과 없음")
				}
				result.SetText(colorOutput(out)).ScrollToBeginning()
			})
		}()
	}
	query.SetDoneFunc(func(k tcell.Key) {
		if k == tcell.KeyEnter {
			apply()
		}
	})
	page.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch e.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			u.cycleFocus(page, e.Key())
			return nil
		case tcell.KeyF9:
			u.close()
			return nil
		case tcell.KeyPgUp, tcell.KeyPgDn:
			queryResultPane(page).InputHandler()(e, func(p tview.Primitive) { u.app.SetFocus(p) })
			return nil
		}
		return e
	})
	u.forgetPageFocus("dialog")
	u.setFocusOrder(page, func() []tview.Primitive { return []tview.Primitive{view, query, result} })
	u.pages.AddPage("dialog", page, true, true)
	u.app.SetFocus(query)
}

// Copy and paging target the focused pane; query input defaults to the result.
func queryResultPane(page tview.Primitive) *queryView {
	if f, ok := page.(*tview.Flex); ok && f.GetItemCount() == 4 {
		source, sourceOK := f.GetItem(0).(*queryView)
		result, resultOK := f.GetItem(3).(*queryView)
		if sourceOK && resultOK {
			if source.HasFocus() {
				return source
			}
			return result
		}
	}
	return nil
}

// Preserve JSON stdout as a queryable document. Text searches include diagnostics.
func (u *UI) querySource() string {
	if json.Valid([]byte(u.raw)) {
		return u.raw
	}
	if u.raw == "" {
		return garden.Clean(u.rawErr)
	}
	if u.rawErr == "" {
		return garden.Clean(u.raw)
	}
	return garden.Clean(strings.TrimRight(u.raw, "\n") + "\n" + u.rawErr)
}

func (u *UI) resultQuery() {
	if u.busy {
		return
	}
	tool, pkg := "rg", "ripgrep"
	if json.Valid([]byte(u.querySource())) {
		tool, pkg = "jq", "jq"
	}
	if _, e := exec.LookPath(tool); e != nil {
		u.alert(fmt.Sprintf("%s가 없습니다. 터미널에서 sudo apt install %s 실행 후 다시 시도하세요.", tool, pkg))
		return
	}
	u.openResultQuery()
}
