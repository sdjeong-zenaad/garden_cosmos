package garden

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshProbesVersionOnce(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "mock")
	calls := filepath.Join(dir, "calls")
	t.Setenv("GCOSM_PROBE_LOG", calls)
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$GCOSM_PROBE_LOG"
case "$1" in
 version) echo v1 ;;
 --help) printf 'App\nUsage:\n  mock [flags]\n' ;;
 *) exit 1 ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		force bool
		want  string
	}{
		{"cold", false, "version\n--help\n"},
		{"cached", false, "version\n"},
		{"forced", true, "version\n--help\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(calls, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Refresh(context.Background(), dir, Profile{Binary: binary}, tc.force, nil); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(calls)
			if err != nil || string(got) != tc.want {
				t.Fatalf("calls %q, want %q: %v", strings.TrimSpace(string(got)), tc.want, err)
			}
		})
	}
}
