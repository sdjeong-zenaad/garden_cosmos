package garden

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSessionCounts(t *testing.T) {
	s := &Session{}
	s.Record("/a/cli", []string{"query", "bank"}, true, nil, false)
	s.Record("/a/cli", []string{"query", "bank"}, true, fmt.Errorf("error"), false)
	s.Record("/a/cli", []string{"tx", "send"}, true, fmt.Errorf("killed"), true)
	s.Record("/a/cli", []string{"version"}, false, fmt.Errorf("not found"), false)
	if s.Total != 3 || len(s.Commands) != 2 || s.ExitZero != 1 || s.Failed != 1 || s.Canceled != 1 || s.StartFailures != 1 {
		t.Fatal(s)
	}
	var b bytes.Buffer
	s.Summary(&b)
	if !strings.Contains(b.String(), "2종류 · 총 3회") || !strings.Contains(b.String(), "/a/cli query bank: 2회") {
		t.Fatal(b.String())
	}
}
func TestNoCommandsAndProcessStart(t *testing.T) {
	s := &Session{}
	var b bytes.Buffer
	s.Summary(&b)
	if !strings.Contains(b.String(), "0종류 · 총 0회") {
		t.Fatal(b.String())
	}
	r := Run(context.Background(), "/nonexistent/gcosm-test", nil)
	if r.Started {
		t.Fatal("missing binary counted as started")
	}
	r = Run(context.Background(), "/bin/false", nil)
	if !r.Started || r.ExitCode != 1 {
		t.Fatal(r)
	}
}
