package garden

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RunInteractive lets util-linux script manage the PTY, raw input, resize and
// terminal restoration. Capture only its output stream, never raw keystrokes.
// No transcript is written to disk; exceeding the capture limit does not stop
// the command or truncate the live terminal output.
func RunInteractive(binary string, args []string, input *os.File, output io.Writer) Result {
	start := time.Now()
	helper, err := exec.LookPath("script")
	if err != nil {
		return Result{Err: fmt.Errorf("대화형 실행 도구가 없습니다: sudo apt install util-linux"), ExitCode: -1}
	}
	if _, err = exec.LookPath(binary); err != nil {
		return Result{Err: err, ExitCode: -1}
	}
	c := exec.Command(helper, "--quiet", "--return", "--flush", "--command", "exec "+Preview(binary, args), "/dev/null")
	c.Env = append(os.Environ(), "SHELL=/bin/sh")
	c.Stdin = input
	var transcript, diagnostics cappedBuffer
	live := &interactiveWriter{writer: output}
	c.Stdout = io.MultiWriter(&transcript, live)
	c.Stderr = io.MultiWriter(&diagnostics, live)
	c.WaitDelay = time.Second
	err = c.Run()
	code := 0
	if err != nil {
		code = -1
		if c.ProcessState != nil {
			code = c.ProcessState.ExitCode()
		}
	}
	return Result{Stdout: Clean(strings.ReplaceAll(transcript.String(), "\r\n", "\n")), Stderr: Clean(diagnostics.String()), ExitCode: code, Duration: time.Since(start), Truncated: transcript.truncated || diagnostics.truncated, Err: err, Started: c.Process != nil}
}

// os/exec may copy stdout and helper diagnostics concurrently.
type interactiveWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *interactiveWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}
