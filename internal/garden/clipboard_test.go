package garden

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClipboardDetection(t *testing.T) {
	for _, tc := range []struct {
		wayland, display string
		tools            []string
		want             string
	}{{"wayland-0", ":0", []string{"wl-copy", "wl-paste", "xclip"}, "Wayland"}, {"wayland-0", ":0", []string{"xclip"}, "X11"}, {"", ":0", []string{"xclip"}, "X11"}, {"", "", []string{"xclip"}, ""}, {"wayland-0", "", []string{"wl-copy"}, ""}} {
		env := func(k string) string {
			if k == "WAYLAND_DISPLAY" {
				return tc.wayland
			}
			return tc.display
		}
		find := func(name string) (string, error) {
			for _, v := range tc.tools {
				if name == v {
					return "/bin/" + name, nil
				}
			}
			return "", fmt.Errorf("missing")
		}
		c, e := detectClipboard(env, find)
		if c.Name != tc.want || (e != nil) != (tc.want == "") {
			t.Fatal(tc, c, e)
		}
		if c.Name == "X11" && !reflect.DeepEqual(c.PasteArgs, []string{"-selection", "clipboard", "-out"}) {
			t.Fatal(c)
		}
	}
}
func TestClipboardRoundtripWithoutDesktop(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "clipboard")
	write := filepath.Join(dir, "copy")
	read := filepath.Join(dir, "paste")
	os.WriteFile(write, []byte("#!/bin/sh\n/bin/cat > '"+data+"'\n"), 0700)
	os.WriteFile(read, []byte("#!/bin/sh\n/bin/cat '"+data+"'\n"), 0700)
	c := Clipboard{Name: "mock", Copy: write, Paste: read}
	want := "한글\n{\"hash\":\"$(not a command)\"}\n"
	if e := c.Write(context.Background(), want); e != nil {
		t.Fatal(e)
	}
	got, e := c.Read(context.Background())
	if e != nil || got != want {
		t.Fatal(got, e)
	}
	if e = c.Write(context.Background(), strings.Repeat("x", OutputLimit+1)); e == nil {
		t.Fatal("copy limit missing")
	}
	os.WriteFile(data, []byte{0xff, 0}, 0600)
	if _, e = c.Read(context.Background()); e == nil {
		t.Fatal("binary accepted")
	}
}
func TestClipboardTimeout(t *testing.T) {
	c := Clipboard{Name: "mock", Paste: "/bin/sleep", PasteArgs: []string{"10"}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, e := c.Read(ctx); e == nil || time.Since(start) > time.Second {
		t.Fatal("timeout not enforced", e)
	}
}
func TestClipboardFieldText(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		valid       bool
	}{{"address\r\n", "address", true}, {"{\n\"value\": 42\n}", `{"value":42}`, true}, {"line1\nline2", "", false}, {"\x1b[31m", "", false}, {"$(echo hi)", "$(echo hi)", true}} {
		got, e := ClipboardFieldText(tc.input)
		if (e == nil) != tc.valid || tc.valid && got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
}

func TestKDEClipboardFallback(t *testing.T) {
	env := func(k string) string {
		switch k {
		case "XDG_CURRENT_DESKTOP":
			return "KDE"
		case "DBUS_SESSION_BUS_ADDRESS":
			return "unix:path=/test/bus"
		case "WAYLAND_DISPLAY":
			return "wayland-0"
		}
		return ""
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "qdbus6")
	data := filepath.Join(dir, "data")
	script := "#!/bin/sh\ncase \"$3\" in\n*.setClipboardContents) printf '%s' \"$4\" > '" + data + "';;\n*.getClipboardContents) /bin/cat '" + data + "'; printf '\\n';;\n*) exit 1;;\nesac\n"
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	find := func(name string) (string, error) {
		if name == "qdbus6" {
			return helper, nil
		}
		return "", fmt.Errorf("missing")
	}
	c, err := detectClipboard(env, find)
	if err != nil || c.Name != "KDE Klipper" {
		t.Fatal(c, err)
	}
	for _, want := range []string{"한글 'quote' $(not executed)", "line\n\n", ""} {
		if err := c.Write(context.Background(), want); err != nil {
			t.Fatal(err)
		}
		got, err := c.Read(context.Background())
		if err != nil || got != want {
			t.Fatalf("roundtrip %q != %q: %v", got, want, err)
		}
	}
}
