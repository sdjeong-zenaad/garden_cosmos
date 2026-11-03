package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	calculatorPage       tview.Primitive
	terminalClipboard    bool
	sshInputModes        bool
	terminalMode         bool
	modeOrigin           tview.Primitive
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
	u.build()
	u.sshInputModes = garden.SSHSession() && mouse
	if garden.SSHSession() {
		_, err := garden.DetectClipboard()
		u.terminalClipboard = err != nil
	}
	u.app.SetRoot(u.screenRoot(), true).EnableMouse(mouse).EnablePaste(true)
	u.installInput()
	u.installStartup()
	return u.app.Run()
}
func (u *UI) installInput() {
	u.app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		u.syncInputMode()
		defer u.syncInputMode()
		if u.sshInputModes && (e.Key() == tcell.KeyF3 || e.Key() == tcell.KeyF4) {
			u.setTerminalMode(!u.terminalMode)
			return nil
		}
		if e.Key() == tcell.KeyF10 {
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
			u.app.Stop()
			return nil
		}
		if e.Key() == tcell.KeyF3 || e.Key() == tcell.KeyF4 {
			key := tcell.KeyCtrlY
			if e.Key() == tcell.KeyF4 {
				key = tcell.KeyCtrlV
			}
			u.clipboardShortcut(key)
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
		case tcell.KeyBacktab:
			if u.app.GetFocus() == u.search {
				u.app.SetFocus(u.output)
			} else if u.app.GetFocus() == u.output {
				u.app.SetFocus(u.list)
			} else {
				u.app.SetFocus(u.search)
			}
			return nil
		case tcell.KeyTab:
			if u.app.GetFocus() == u.search {
				u.app.SetFocus(u.list)
			} else if u.app.GetFocus() == u.list {
				u.app.SetFocus(u.output)
			} else {
				u.app.SetFocus(u.search)
			}
			return nil
		}
		return e
	})
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
	u.list.SetSelectedFunc(func(int, string, string, rune) { u.commandForm() })
	left := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.search, 1, 0, true).AddItem(u.list, 0, 1, false)
	right := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.help, 0, 1, false).AddItem(u.output, 0, 2, false)
	body := tview.NewFlex().AddItem(left, 0, 1, true).AddItem(right, 0, 2, false)
	root := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.status, 2, 0, false).AddItem(body, 0, 1, true)
	u.pages.AddPage("main", root, true, true)
	u.message("gcosm · CLI를 선택하세요 (F1)")
}
func (u *UI) message(s string) { u.status.SetText(tview.Escape(garden.Clean(s))) }
func (u *UI) close() {
	u.calculatorPage = nil
	u.pages.RemovePage("dialog")
	if u.editor != nil {
		u.app.SetFocus(u.editor.form)
	} else {
		u.app.SetFocus(u.list)
	}
}
func (u *UI) modal(p tview.Primitive, w, h int) {
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
	u.pages.AddPage("dialog", outer, true, true)
	u.app.SetFocus(p)
}
func (u *UI) alert(s string) {
	m := newActionModal().SetText(tview.Escape(s)).AddButtons([]string{"확인"}).SetDoneFunc(func(int, string) { u.close() })
	u.modal(m, 76, 14)
}
func (u *UI) settings() {
	if u.busy {
		u.message("작업 중에는 CLI 설정을 바꿀 수 없습니다.")
		return
	}
	form := tview.NewForm()
	form.AddInputField("CLI 경로", u.profile.Binary, 54, nil, nil).AddInputField("체인 home (선택)", u.profile.Home, 54, nil, nil)
	field := form.GetFormItem(0).(*tview.InputField)
	styleAutocomplete(field)
	field.SetAutocompleteFunc(func(s string) []string {
		if s == "" {
			return nil
		}
		matches, _ := filepath.Glob(s + "*")
		if len(matches) > 30 {
			matches = matches[:30]
		}
		return matches
	})
	form.AddButton("저장 및 분석", func() {
		p := garden.Profile{Binary: strings.TrimSpace(field.GetText()), Home: strings.TrimSpace(form.GetFormItem(1).(*tview.InputField).GetText())}
		binary, _, e := garden.Identity(p.Binary)
		if e != nil {
			u.alert(e.Error())
			return
		}
		p.Binary = binary
		if p.Home != "" {
			p.Home, e = filepath.Abs(p.Home)
			if e != nil {
				u.alert(e.Error())
				return
			}
		}
		if e = garden.Save(filepath.Join(u.dir, "profile.json"), p); e != nil {
			u.alert(e.Error())
			return
		}
		u.profile = p
		u.descriptor = nil
		u.selected = nil
		u.list.Clear()
		u.close()
		u.app.SetFocus(u.search)
		u.refresh(false)
	})
	form.AddButton("현재 CLI 재분석", func() {
		if u.profile.Binary == "" {
			return
		}
		if u.editor != nil {
			u.leaveCommand()
			if u.editor != nil {
				return
			}
		}
		u.close()
		u.refresh(true)
	})
	form.AddButton("취소", u.close).SetBorder(true).SetTitle(" CLI 설정 · Esc 닫기 ")
	u.modal(form, 84, 12)
}
func (u *UI) refresh(force bool) {
	if u.busy {
		return
	}
	if u.profile.Binary == "" {
		u.settings()
		return
	}
	u.busy = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	u.cancel = cancel
	u.message("CLI 도움말 분석 중… Ctrl+C 취소")
	profile := u.profile
	go func() {
		d, e := garden.Refresh(ctx, u.dir, profile, force, func(s string) { u.app.QueueUpdateDraw(func() { u.message("분석 중: " + s + " · Ctrl+C 취소") }) })
		cancel()
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			u.cancel = nil
			if u.quitRequested {
				u.app.Stop()
				return
			}
			if e != nil {
				u.descriptor = nil
				u.selected = nil
				u.list.Clear()
				u.alert("분석 실패: " + e.Error())
				return
			}
			u.descriptor = d
			u.populate(u.search.GetText())
			u.message(fmt.Sprintf("%s · %s · %d 명령 · 경고 %d · F6 설정/진단", filepath.Base(d.Binary), d.Version, len(garden.Flatten(d.Root)), len(d.Warnings)))
		})
	}()
}
func (u *UI) commands() []*garden.Command {
	if u.descriptor == nil {
		return nil
	}
	q := strings.ToLower(u.search.GetText())
	var out []*garden.Command
	for _, c := range garden.Flatten(u.descriptor.Root) {
		// Keep command groups in the descriptor for traversal, but do not
		// offer help-only parents as executable commands in the browser.
		if len(c.Children) > 0 || strings.Contains(c.Usage, "[command]") {
			continue
		}
		if strings.Contains(strings.ToLower(strings.Join(c.Path, " ")+" "+c.Summary), q) {
			out = append(out, c)
		}
	}
	return out
}
func (u *UI) populate(string) {
	u.list.Clear()
	for _, c := range u.commands() {
		name := strings.Join(c.Path, " ")
		if name == "" {
			name = "(root)"
		}
		u.list.AddItem(name, c.Summary, 0, nil)
	}
	u.selectIndex(0)
}
func (u *UI) selectIndex(i int) {
	cs := u.commands()
	if i < 0 || i >= len(cs) {
		u.selected = nil
		u.help.SetText("")
		return
	}
	u.selected = cs[i]
	u.help.SetText(u.selected.Help)
	u.help.ScrollToBeginning()
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
func (u *UI) confirmArgs(interactive bool, args, path []string) {
	if u.busy || u.descriptor == nil {
		return
	}

	binary := u.descriptor.Binary
	fingerprint := u.descriptor.Fingerprint
	text := "다음 명령을 실행합니다. 체인·계정·수수료를 확인하세요.\n\n" + garden.Preview(binary, args) + "\n\n캡처 실행: stdin 없음, 최대 2분 / 각 출력 4 MiB\n대화형: 터미널 입력 가능, 통합 출력 최대 4 MiB를 결과 창에 표시\n트랜잭션 전송 후 취소해도 전송은 되돌릴 수 없습니다."
	m := newActionModal().SetText(tview.Escape(text)).AddButtons([]string{"취소", "실행"}).SetDoneFunc(func(i int, _ string) {
		u.close()
		if i != 1 {
			return
		}
		_, hash, e := garden.Identity(binary)
		if e != nil || hash != fingerprint {
			u.alert("CLI 실행 파일이 바뀌었거나 읽을 수 없습니다. F5로 다시 분석하세요.")
			return
		}
		if e := u.saveCommon(); e != nil {
			u.alert(e.Error())
			return
		}
		u.leaveCommand()
		if u.editor != nil {
			return
		}
		if interactive {
			u.interactive(binary, args, path)
			return
		}
		u.execute(binary, args, path)
	})
	u.modal(m, 90, 22)
}
func (u *UI) execute(binary string, args, path []string) {
	u.busy = true
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	u.cancel = cancel
	u.message("실행 중… Ctrl+C 취소 · " + garden.Preview(binary, args))
	go func() {
		r := garden.Run(ctx, binary, args)
		canceled := ctx.Err() != nil
		cancel()
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			u.cancel = nil
			u.session.Record(binary, path, r.Started, r.Err, canceled)
			u.raw = r.Stdout
			u.rawErr = r.Stderr
			u.render(r.Stdout, r.Stderr)
			msg := fmt.Sprintf("종료 코드 %d · %s", r.ExitCode, r.Duration.Round(time.Millisecond))
			if r.Err != nil {
				msg += " · " + r.Err.Error()
			}
			if r.Truncated {
				msg += " · 출력 크기 제한으로 잘림"
			}
			u.message(msg)
			if !r.Truncated {
				u.store.Capture(r.Stdout, path)
				u.store.Capture(r.Stderr, path)
				if e := garden.Save(filepath.Join(u.dir, "bookmarks.json"), u.store); e != nil {
					u.message(msg + " · 캐시 저장 실패: " + e.Error())
				}
			}
			if u.quitRequested {
				u.app.Stop()
				return
			}
			u.app.SetFocus(u.output)
		})
	}()
}
func (u *UI) render(stdout, stderr string) {
	u.displayOutput = garden.Pretty(stdout)
	out := "[green]STDOUT[-]\n" + colorOutput(stdout)
	if stderr != "" {
		out += "\n\n[red]STDERR[-]\n" + tview.Escape(garden.Clean(stderr))
	}
	u.output.SetText(out)
	u.output.ScrollToBeginning()
}
func (u *UI) interactive(binary string, args, path []string) {
	if _, err := exec.LookPath("script"); err != nil {
		u.alert("대화형 실행 도구가 없습니다: sudo apt install util-linux")
		return
	}
	var result garden.Result
	if !u.app.Suspend(func() {
		fmt.Printf("대화형 실행 시작: %s\n", garden.Preview(binary, args))
		result = garden.RunInteractive(binary, args, os.Stdin, os.Stdout)
		if result.Err != nil {
			fmt.Printf("\n대화형 실행 종료: %v\n", result.Err)
		} else {
			fmt.Println("\n대화형 실행 종료: 종료 코드 0")
		}
		fmt.Print("Enter를 누르면 결과 창으로 돌아갑니다.")
		var line string
		fmt.Scanln(&line)
	}) {
		u.message("터미널을 대화형 모드로 전환하지 못했습니다.")
		return
	}
	u.finishInteractive(binary, path, result)
}
func (u *UI) finishInteractive(binary string, path []string, r garden.Result) {
	u.session.Record(binary, path, r.Started, r.Err, r.ExitCode == 130 || r.ExitCode == 143)
	u.raw = r.Stdout
	u.rawErr = r.Stderr
	u.displayOutput = garden.Pretty(r.Stdout)
	out := "[green]대화형 출력 · stdout/stderr 통합[-]\n" + colorOutput(r.Stdout)
	if r.Stderr != "" {
		out += "\n\n[red]실행 도구 메시지[-]\n" + tview.Escape(r.Stderr)
	}
	u.output.SetText(out).ScrollToBeginning()
	message := fmt.Sprintf("대화형 실행 종료 · 종료 코드 %d · %s", r.ExitCode, r.Duration.Round(time.Millisecond))
	if r.Truncated {
		message += " · 출력 4 MiB 제한으로 잘림"
	} else {
		u.store.Capture(r.Stdout, path)
		if err := garden.Save(filepath.Join(u.dir, "bookmarks.json"), u.store); err != nil {
			message += " · 캐시 저장 실패: " + err.Error()
		}
	}
	if r.Err != nil {
		message += " · " + r.Err.Error()
	}
	u.message(message)
	u.app.SetFocus(u.output)
}
func (u *UI) configs() {
	if u.descriptor == nil {
		return
	}
	b, _ := json.MarshalIndent(struct {
		Home     string
		Configs  []garden.Config
		Warnings []string
	}{u.descriptor.Home, u.descriptor.Configs, u.descriptor.Warnings}, "", "  ")
	v := tview.NewTextView().SetText(string(b)).SetScrollable(true)
	v.SetBorder(true).SetTitle(" 기존 TOML 설정 / 분석 경고 · Esc 닫기 ")
	u.modal(v, 100, 32)
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

func (u *UI) bat() {
	if u.busy {
		return
	}
	binary, e := exec.LookPath("batcat")
	if e != nil {
		u.alert("batcat이 없습니다. 터미널에서 sudo apt install bat 실행 후 다시 시도하세요.")
		return
	}
	raw := u.raw
	u.app.Suspend(func() {
		c := exec.Command(binary, "--paging=always", "--style=numbers", "--language=json", "--color=always")
		c.Stdin = strings.NewReader(garden.Clean(raw))
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		_ = c.Run()
	})
}
