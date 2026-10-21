package garden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const OutputLimit = 4 * 1024 * 1024

type cappedBuffer struct {
	bytes.Buffer
	truncated bool
}

// Prevent io.Copy from bypassing Write's limit through bytes.Buffer.ReadFrom.
func (b *cappedBuffer) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{b}, r)
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := OutputLimit - b.Len()
	if len(p) > left {
		p = p[:left]
		b.truncated = true
	}
	_, e := b.Buffer.Write(p)
	return n, e
}

type Result struct {
	Stdout, Stderr string
	ExitCode       int
	Duration       time.Duration
	Truncated      bool
	Err            error
	Started        bool
}

func Run(ctx context.Context, binary string, args []string) Result {
	start := time.Now()
	c := exec.CommandContext(ctx, binary, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
	c.WaitDelay = time.Second
	var out, errout cappedBuffer
	c.Stdout = &out
	c.Stderr = &errout
	e := c.Run()
	code := 0
	if e != nil {
		code = -1
		if c.ProcessState != nil {
			code = c.ProcessState.ExitCode()
		}
	}
	return Result{out.String(), errout.String(), code, time.Since(start), out.truncated || errout.truncated, e, c.Process != nil}
}
func Probe(ctx context.Context, binary string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	r := Run(ctx, binary, args)
	if r.Err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(args, " "), r.Err, r.Stderr)
	}
	if r.Truncated {
		return "", fmt.Errorf("help output exceeded limit")
	}
	return r.Stdout, nil
}

var ansi = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

func Clean(s string) string {
	s = ansi.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		if r == 127 {
			return -1
		}
		return r
	}, s)
}
func Pretty(s string) string {
	var b bytes.Buffer
	if json.Indent(&b, []byte(s), "", "  ") == nil {
		return b.String()
	}
	return Clean(s)
}

var shellWord = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func Preview(binary string, args []string) string {
	all := append([]string{binary}, args...)
	for i, s := range all {
		if !shellWord.MatchString(s) {
			all[i] = "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
		}
	}
	return strings.Join(all, " ")
}

// Captured mode never provides stdin. Interactive mode is explicitly selected in the UI.
func Sensitive(path []string) bool {
	for _, p := range path {
		if p == "keys" || p == "export" || p == "init" || p == "genesis" {
			return true
		}
	}
	return false
}

var candidates = regexp.MustCompile(`\b(?:[a-z][a-z0-9]{1,30}1[023456789acdefghjklmnpqrstuvwxyz]{20,90}|0x(?:[0-9a-fA-F]{64}|[0-9a-fA-F]{40})|[0-9a-fA-F]{64})\b`)

func (s *Store) Capture(output string, path []string) {
	if Sensitive(path) {
		return
	}
	for _, v := range candidates.FindAllString(output, -1) {
		kind := "address"
		if len(v) == 64 || len(v) == 66 {
			kind = "hash"
		}
		s.Put(v, kind, "", "")
	}
}

func FilterJSON(ctx context.Context, input, filter string) (string, error) {
	c := exec.CommandContext(ctx, "jq", "-M", filter)
	c.Stdin = strings.NewReader(input)
	var out, errout cappedBuffer
	c.Stdout = &out
	c.Stderr = &errout
	if e := c.Run(); e != nil {
		return "", fmt.Errorf("jq: %w: %s", e, errout.String())
	}
	if out.truncated {
		return "", fmt.Errorf("jq output exceeds 4 MiB")
	}
	return out.String(), nil
}

// FilterText passes a single ERE pattern to grep, never through a shell.
// Exit status 1 means no matches, not a query failure.
func FilterText(ctx context.Context, input, pattern string) (string, error) {
	c := exec.CommandContext(ctx, "grep", "-a", "-E", "--color=never", "-e", pattern)
	c.Stdin = strings.NewReader(input)
	var out, errout cappedBuffer
	c.Stdout, c.Stderr = &out, &errout
	c.WaitDelay = time.Second
	if err := c.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			return "", fmt.Errorf("grep: %w: %s", err, errout.String())
		}
	}
	if out.truncated {
		return "", fmt.Errorf("grep output exceeds 4 MiB")
	}
	return out.String(), nil
}
