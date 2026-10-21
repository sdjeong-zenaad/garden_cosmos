package garden

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPreviewShellWords(t *testing.T) {
	if got := Preview("xplad", []string{"keys", "list", "--output=json"}); got != "xplad keys list --output=json" {
		t.Fatal(got)
	}
	args := []string{"plain", "with space", "O'Brien", "$(printf BAD)", "", "line\nbreak", "한글"}
	command := Preview("printf", append([]string{"%s\\0"}, args...))
	out, err := exec.Command("/bin/sh", "-c", command).Output()
	if err != nil {
		t.Fatal(err, command)
	}
	if string(out) != strings.Join(args, "\x00")+"\x00" {
		t.Fatalf("shell round trip failed: %q / %s", out, command)
	}
}
