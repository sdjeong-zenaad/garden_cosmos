package garden

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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

// FilterText passes a single ripgrep regex via stdin, never through a shell.
// Ignore user configuration so searches cannot expand to files or change formatting.
// Exit status 1 means no matches, not a query failure.
func FilterText(ctx context.Context, input, pattern string) (string, error) {
	c := exec.CommandContext(ctx, "rg", "--no-config", "--text", "--color=never", "--no-heading", "--no-filename", "--no-line-number", "-e", pattern, "-")
	c.Stdin = strings.NewReader(input)
	var out, errout cappedBuffer
	c.Stdout, c.Stderr = &out, &errout
	c.WaitDelay = time.Second
	if err := c.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			return "", fmt.Errorf("rg: %w: %s", err, errout.String())
		}
	}
	if out.truncated {
		return "", fmt.Errorf("rg output exceeds 4 MiB")
	}
	return out.String(), nil
}
