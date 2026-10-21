package garden

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const help = `App

Usage:
  mock [command]

Available Commands:
  query       Query chain
  keys        Manage keys
  help        Help

Flags:
  -h, --help                help for mock
      --home string         config directory (default "/tmp/mock")
      --resolve-denom

Global Flags:
      --chain-id string     network
`

func TestParseHelp(t *testing.T) {
	c := ParseHelp(nil, help)
	if len(c.Children) != 2 || c.Usage != "mock [command]" {
		t.Fatalf("%+v", c)
	}
	if len(c.Flags) != 4 {
		t.Fatalf("flags: %+v", c.Flags)
	}
	if c.Flags[1].Default != "/tmp/mock" || c.Flags[2].Type != "bool" || !c.Flags[3].Global {
		t.Fatal(c.Flags)
	}
}
func TestRunAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r := Run(ctx, "sh", []string{"-c", "printf before; printf error >&2; sleep 10"})
	if r.Stdout != "before" || r.Stderr != "error" || r.Err == nil || r.Duration > 2*time.Second {
		t.Fatalf("%+v", r)
	}
	r = Run(context.Background(), "printf", []string{"%s", "$(touch /tmp/garden-injection)"})
	if r.Stdout != "$(touch /tmp/garden-injection)" {
		t.Fatal(r)
	}
}
func TestOutputLimit(t *testing.T) {
	var b cappedBuffer
	p := make([]byte, OutputLimit+10)
	n, e := b.Write(p)
	if n != len(p) || e != nil || !b.truncated || b.Len() != OutputLimit {
		t.Fatal(n, e, b.Len())
	}
}
func TestStoreAndJSON5(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	os.WriteFile(p, []byte(`{ // comment
 Entries: [{Value: 'abc', Kind: 'manual',}],
}`), 0600)
	var s Store
	if e := Load(p, &s); e != nil {
		t.Fatal(e)
	}
	s.Put("abc", "manual", "Alice", "memo")
	s.Put("abc", "manual", "", "")
	if len(s.Entries) != 1 || s.Entries[0].Nickname != "Alice" {
		t.Fatal(s)
	}
	if e := Save(p, s); e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}
func TestCapture(t *testing.T) {
	var s Store
	hash := strings.Repeat("AB", 32)
	s.Capture(hash, []string{"keys", "add"})
	if len(s.Entries) != 0 {
		t.Fatal("cached sensitive output")
	}
	s.Capture(hash+" "+hash+" 0x"+strings.Repeat("a", 40), []string{"query"})
	if len(s.Entries) != 2 {
		t.Fatal(s)
	}
	s.Put(hash, "hash", "tx", "keep")
	s.Capture(hash, []string{"query"})
	if s.Entries[0].Note != "keep" {
		t.Fatal(s)
	}
}
func TestConfigs(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "config"), 0700)
	os.WriteFile(filepath.Join(dir, "config", "client.toml"), []byte("chain-id = 'test'\n[auth]\ntoken='hidden'\n"), 0600)
	os.WriteFile(filepath.Join(dir, "config", "app.toml"), []byte("not = [valid"), 0600)
	c := ReadConfigs(dir)
	if len(c) != 2 || c[0].Values["chain-id"] != "test" || c[0].Values["auth"].(map[string]any)["token"] != "[redacted]" || c[1].Error == "" {
		t.Fatalf("%+v", c)
	}
}
func TestRefresh(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "mock")
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\nif [ \"$1\" = version ]; then echo v1; exit 0; fi\nif [ \"$*\" = --help ]; then printf 'App\\nUsage:\\n  mock [command]\\nAvailable Commands:\\n  query       Query\\n'; exit 0; fi\nif [ \"$*\" = 'query --help' ]; then printf 'Query\\nUsage:\\n  mock query [flags]\\n'; exit 0; fi\nexit 9\n"
	os.WriteFile(binary, []byte(script), 0700)
	p := Profile{Binary: binary}
	d, e := Refresh(context.Background(), dir, p, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(Flatten(d.Root)) != 2 {
		t.Fatal(d)
	}
	os.WriteFile(log, nil, 0600)
	_, e = Refresh(context.Background(), dir, p, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(log)
	if strings.Contains(string(b), "--help") {
		t.Fatal("cache not reused", string(b))
	}
	os.WriteFile(binary, []byte(script+"# rebuild\n"), 0700)
	_, e = Refresh(context.Background(), dir, p, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(log)
	if !strings.Contains(string(b), "--help") {
		t.Fatal("binary change ignored")
	}
}
func TestClean(t *testing.T) {
	if got := Clean("\x1b[31mred\x1b[0m\x1b]52;c;secret\a\x00"); got != "red" {
		t.Fatal(got)
	}
}
