package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"garden_cosmos/internal/garden"
)

func TestNoArgsWithoutCLIShowsHelp(t *testing.T) {
	for _, emptyProfile := range []bool{false, true} {
		name := "missing profile"
		if emptyProfile {
			name = "empty profile"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			dir, err := garden.DataDir()
			if err != nil {
				t.Fatal(err)
			}
			if emptyProfile {
				if err := garden.Save(filepath.Join(dir, "profile.json"), garden.Profile{}); err != nil {
					t.Fatal(err)
				}
			}
			oldArgs, oldOut := os.Args, os.Stdout
			defer func() { os.Args = oldArgs; os.Stdout = oldOut }()
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			os.Args = []string{"gcosm"}
			os.Stdout = w
			err = run()
			w.Close()
			out, _ := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "사용법: gcosm") || !strings.Contains(string(out), "--cli /absolute/path/to/xplad") {
				t.Fatalf("missing help: %s", out)
			}
			if strings.Contains(string(out), "실행한 명령:") {
				t.Fatal("started session")
			}
			if _, err := os.Stat(filepath.Join(dir, ".lock")); !os.IsNotExist(err) {
				t.Fatal("unexpected TUI lock")
			}
			if !emptyProfile {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatal("help created data directory")
				}
			}
		})
	}
}

func TestDoctorMissingRipgrep(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	old := os.Stdout
	defer func() { os.Stdout = old }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	os.Stdout = w
	Doctor()
	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "rg: 미설치 (선택 도구)") || !strings.Contains(out, "sudo apt install jq bat ripgrep util-linux") {
		t.Fatal(out)
	}
}
