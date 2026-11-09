package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"garden_cosmos/internal/garden"

	"github.com/rivo/tview"
)

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
			u.app.SetFocus(u.search)
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
	u.app.SetFocus(u.search)
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
