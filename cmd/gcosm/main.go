package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"garden_cosmos/internal/terminal"
	"golang.org/x/term"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"garden_cosmos/internal/garden"
	"garden_cosmos/internal/ui"
)

var version = "0.10.0"

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "gcosm:", e)
		os.Exit(1)
	}
}
func run() error {
	dir, e := garden.DataDir()
	if e != nil {
		return e
	}
	fs := flag.NewFlagSet("gcosm", flag.ContinueOnError)
	fs.StringVar(&dir, "data-dir", dir, "descriptor와 북마크 저장 위치")
	binary := fs.String("cli", "", "Cosmos CLI 경로")
	home := fs.String("home", "", "체인의 home 디렉터리 (기존 TOML 읽기)")
	analyze := fs.Bool("analyze", false, "도움말 분석 후 종료")
	force := fs.Bool("refresh", false, "descriptor 강제 재분석")
	doctor := fs.Bool("doctor", false, "선택 도구 설치 상태 확인")
	ver := fs.Bool("version", false, "버전 출력")
	mouse := fs.Bool("mouse", true, "시작 시 앱 마우스 사용 (F3: 사용, F4: 해제)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "사용법: gcosm [옵션]")
		fmt.Fprintln(fs.Output(), "첫 실행: gcosm --cli /absolute/path/to/xplad")
		fmt.Fprintln(fs.Output(), "인자 없이 실행하면 마지막 CLI 설정으로 TUI를 시작합니다.")
		fs.PrintDefaults()
	}
	if e = fs.Parse(os.Args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if len(os.Args) == 1 {
		var saved garden.Profile
		if e = garden.Load(filepath.Join(dir, "profile.json"), &saved); e != nil && !os.IsNotExist(e) {
			return e
		}
		if saved.Binary == "" {
			fs.SetOutput(os.Stdout)
			fs.Usage()
			return nil
		}
	}
	if *ver {
		fmt.Println(version)
		return nil
	}
	if *doctor {
		Doctor()
		return nil
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fmt.Errorf("같은 데이터 폴더를 사용하는 gcosm이 실행 중입니다: %w", e)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	var p garden.Profile
	if e = garden.Load(filepath.Join(dir, "profile.json"), &p); e != nil && !os.IsNotExist(e) {
		return e
	}
	if *binary != "" {
		p.Binary = *binary
		p.Home = *home
	} else if *home != "" {
		p.Home = *home
	}
	if *binary != "" || *home != "" {
		resolved, _, e := garden.Identity(p.Binary)
		if e != nil {
			return e
		}
		p.Binary = resolved
		if p.Home != "" {
			p.Home, e = filepath.Abs(p.Home)
			if e != nil {
				return e
			}
		}
		if e = garden.Save(filepath.Join(dir, "profile.json"), p); e != nil {
			return e
		}
	}
	if *analyze || *force {
		if p.Binary == "" {
			return fmt.Errorf("--cli 경로를 지정하세요")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		d, e := garden.Refresh(ctx, dir, p, *force, func(s string) { fmt.Fprintln(os.Stderr, "분석:", s) })
		if e != nil {
			return e
		}
		fmt.Printf("%s %s: %d commands, %d warnings\n%s\n", d.Binary, d.Version, len(garden.Flatten(d.Root)), len(d.Warnings), filepath.Join(dir, "descriptor.json"))
		if *analyze {
			return nil
		}
	}
	session := &garden.Session{}
	terminal.Banner(os.Stdout, version, term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb")
	defer session.Summary(os.Stdout)
	return ui.Run(dir, session, *mouse)
}
func Doctor() {
	var missing []string
	for _, name := range []string{"jq", "batcat", "rg", "script"} {
		p, e := exec.LookPath(name)
		if e != nil {
			fmt.Println(name + ": 미설치 (선택 도구)")
			if name == "batcat" {
				missing = append(missing, "bat")
			} else if name == "rg" {
				missing = append(missing, "ripgrep")
			} else if name == "script" {
				missing = append(missing, "util-linux")
			} else {
				missing = append(missing, name)
			}
		} else {
			fmt.Println(name + ": " + p)
		}
	}
	if len(missing) > 0 {
		fmt.Println("Ubuntu 설치: sudo apt update && sudo apt install " + strings.Join(missing, " "))
	}
	if c, e := garden.DetectClipboard(); e != nil {
		fmt.Println(e)
	} else {
		fmt.Println("클립보드: " + c.Name)
	}
	fmt.Println("기본 TUI/JSON 정렬은 외부 도구 없이 실행할 수 있습니다.")
}
