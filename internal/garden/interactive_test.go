package garden

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInteractiveTTYAndExit(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var live bytes.Buffer
	r := RunInteractive("/bin/sh", []string{"-c", `test -t 0 && test -t 1 && test -t 2 || exit 99; printf '\033[32mOUT\033[0m\n'; printf 'ERR\n' >&2; exit 7`}, input, &live)
	if !r.Started || r.ExitCode != 7 || r.Err == nil {
		t.Fatalf("result %+v", r)
	}
	if r.Stdout != "OUT\nERR\n" || r.Stderr != "" {
		t.Fatalf("captured %q / %q", r.Stdout, r.Stderr)
	}
	if !strings.Contains(live.String(), "\x1b[32mOUT") {
		t.Fatal("live colors lost")
	}
}

type promptWriter struct {
	bytes.Buffer
	ready chan struct{}
	sent  bool
}

func (w *promptWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if !w.sent && strings.Contains(w.Buffer.String(), "PASSWORD:") {
		w.sent = true
		close(w.ready)
	}
	return n, err
}
func TestInteractiveHiddenInput(t *testing.T) {
	input, feed, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer feed.Close()
	live := &promptWriter{ready: make(chan struct{})}
	done := make(chan Result, 1)
	go func() {
		done <- RunInteractive("/bin/sh", []string{"-c", `stty -echo; printf 'PASSWORD:'; read -r pw; stty echo; test "$pw" = 'hidden-test-value' || exit 8; printf '\nACCEPTED\n'`}, input, live)
	}()
	select {
	case <-live.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("no interactive prompt")
	}
	fmt.Fprintln(feed, "hidden-test-value")
	select {
	case r := <-done:
		if r.Err != nil || !strings.Contains(r.Stdout, "ACCEPTED") {
			t.Fatalf("result %+v", r)
		}
		if strings.Contains(r.Stdout, "hidden-test-value") || strings.Contains(live.String(), "hidden-test-value") {
			t.Fatal("hidden input was captured")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("interactive input did not complete")
	}
}
func TestInteractiveOutputLimitDoesNotStopCommand(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	r := RunInteractive("/bin/sh", []string{"-c", fmt.Sprintf("head -c %d /dev/zero | tr '\\000' x; printf DONE", OutputLimit+100)}, input, io.Discard)
	if r.Err != nil || !r.Truncated || len(r.Stdout) != OutputLimit {
		t.Fatalf("code=%d truncated=%v bytes=%d err=%v", r.ExitCode, r.Truncated, len(r.Stdout), r.Err)
	}
}
