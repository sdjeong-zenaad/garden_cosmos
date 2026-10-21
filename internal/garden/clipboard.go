package garden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

type Clipboard struct {
	Name, Copy, Paste   string
	CopyArgs, PasteArgs []string
}

func DetectClipboard() (Clipboard, error) { return detectClipboard(os.Getenv, exec.LookPath) }
func detectClipboard(env func(string) string, find func(string) (string, error)) (Clipboard, error) {
	if env("WAYLAND_DISPLAY") != "" {
		c, ce := find("wl-copy")
		p, pe := find("wl-paste")
		if ce == nil && pe == nil {
			return Clipboard{"Wayland", c, p, []string{"--type", "text/plain;charset=utf-8"}, []string{"--no-newline", "--type", "text"}}, nil
		}
	}
	if env("DISPLAY") != "" {
		if p, e := find("xclip"); e == nil {
			return Clipboard{"X11", p, p, []string{"-selection", "clipboard", "-in"}, []string{"-selection", "clipboard", "-out"}}, nil
		}
	}
	if strings.Contains(strings.ToUpper(env("XDG_CURRENT_DESKTOP")), "KDE") && env("DBUS_SESSION_BUS_ADDRESS") != "" {
		for _, name := range []string{"qdbus6", "qdbus"} {
			if p, e := find(name); e == nil {
				return Clipboard{"KDE Klipper", p, p,
					[]string{"org.kde.klipper", "/klipper", "org.kde.klipper.klipper.setClipboardContents"},
					[]string{"org.kde.klipper", "/klipper", "org.kde.klipper.klipper.getClipboardContents"}}, nil
			}
		}
	}
	if env("WAYLAND_DISPLAY") != "" {
		return Clipboard{}, fmt.Errorf("Wayland 클립보드 도구가 없습니다: sudo apt install wl-clipboard")
	}
	if env("DISPLAY") != "" {
		return Clipboard{}, fmt.Errorf("X11 클립보드 도구가 없습니다: sudo apt install xclip")
	}
	return Clipboard{}, fmt.Errorf("데스크톱 세션을 찾지 못했습니다. 터미널의 복사·붙여넣기를 사용하세요")
}
func (c Clipboard) Write(ctx context.Context, text string) error {
	if len(text) > OutputLimit {
		return fmt.Errorf("복사 한도는 4 MiB입니다")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	args := append([]string{}, c.CopyArgs...)
	if c.Name == "KDE Klipper" {
		// qdbus accepts the method's QString as one argument, never shell code.
		args = append(args, text)
	}
	cmd := exec.CommandContext(ctx, c.Copy, args...)
	if c.Name != "KDE Klipper" {
		cmd.Stdin = strings.NewReader(text)
	}
	cmd.WaitDelay = 100 * time.Millisecond
	// Clipboard helpers fork an owner process. Do not create output pipes held by that owner.
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("%s 복사 실패: %w (데스크톱 세션 접근 권한을 확인하세요)", c.Name, e)
	}
	return nil
}
func (c Clipboard) Read(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Paste, c.PasteArgs...)
	var out cappedBuffer
	cmd.Stdout = &out
	cmd.WaitDelay = 100 * time.Millisecond
	if e := cmd.Run(); e != nil {
		return "", fmt.Errorf("%s 붙여넣기 실패: %w (텍스트 클립보드와 세션 접근을 확인하세요)", c.Name, e)
	}
	if out.truncated {
		return "", fmt.Errorf("붙여넣기 한도는 4 MiB입니다")
	}
	s := out.String()
	if c.Name == "KDE Klipper" {
		// qdbus adds one output newline; preserve any newlines in the payload.
		s = strings.TrimSuffix(s, "\n")
	}
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return "", fmt.Errorf("UTF-8 텍스트만 붙여넣을 수 있습니다")
	}
	return s, nil
}
func ClipboardFieldText(s string) (string, error) {
	s = strings.TrimRight(s, "\r\n")
	if strings.ContainsAny(s, "\r\n") {
		var out bytes.Buffer
		if json.Compact(&out, []byte(s)) != nil {
			return "", fmt.Errorf("한 줄 입력만 지원합니다. 여러 줄 JSON은 자동 압축할 수 있습니다")
		}
		s = out.String()
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return "", fmt.Errorf("제어 문자는 입력에 붙여넣을 수 없습니다")
		}
	}
	return s, nil
}
