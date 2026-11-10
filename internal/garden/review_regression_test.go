package garden

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshClearedHomeAndOldSchema(t *testing.T) {
	for _, schema := range []int{1, 2, Schema} {
		dir := t.TempDir()
		binary := filepath.Join(dir, "mock")
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = version ]; then echo v1; else printf 'App\\nUsage:\\n  mock [flags]\\nFlags:\\n      --home string   home (default \"/default\")\\n'; fi\n"), 0700); err != nil {
			t.Fatal(err)
		}
		p, hash, err := Identity(binary)
		if err != nil {
			t.Fatal(err)
		}
		if err := Save(filepath.Join(dir, "descriptor.json"), Descriptor{Schema: schema, Binary: p, Fingerprint: hash, Version: "v1", Home: "/old-custom", RequestedHome: "/old-custom", Root: &Command{Usage: "mock [flags]"}}); err != nil {
			t.Fatal(err)
		}
		d, err := Refresh(context.Background(), dir, Profile{Binary: p}, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.Home != "/default" || d.RequestedHome != "" || d.Schema != Schema {
			t.Fatalf("reset reused stale descriptor: %+v", d)
		}
	}
}
func TestVariadicOutsidePlaceholder(t *testing.T) {
	for _, placeholder := range []string{"[signature]...", "<signature>...", "[signature...]"} {
		cmd := &Command{Path: []string{"tx", "multisign"}, Usage: "mock tx multisign [file] [name] " + placeholder + " [flags]", Flags: []Flag{{Name: "output", Type: "string"}}}
		fields := Fields(cmd)
		if len(fields) != 4 || !fields[2].Variadic || fields[2].Name != "signature" {
			t.Fatalf("%s: %+v", placeholder, fields)
		}
		args, err := FormArgs(cmd, fields, map[string]FieldValue{"arg:0": {true, "tx.json"}, "arg:1": {true, "multisig"}, "arg:2": {true, "sig1.json sig2.json"}, "flag:output": {true, "json"}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(args, " ") != "tx multisign tx.json multisig sig1.json sig2.json --output=json" {
			t.Fatal(args)
		}
	}
}
func TestCalculationLimitsAndCancellation(t *testing.T) {
	if _, err := Calculate("1" + strings.Repeat("*1e-1000", 100)); err == nil {
		t.Fatal("unbounded rational accepted")
	}
	if _, err := Calculate("1e1000*1e1000"); err != nil {
		t.Fatal("reasonable exact result rejected", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CalculateContext(ctx, "1+2"); err != context.Canceled {
		t.Fatal(err)
	}
}
