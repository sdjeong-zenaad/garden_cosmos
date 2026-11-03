package garden

import (
	"strings"
	"testing"
)

func TestSSHSession(t *testing.T) {
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		t.Setenv(key, "")
	}
	if SSHSession() {
		t.Fatal("local detected as SSH")
	}
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		t.Setenv(key, "session")
		if !SSHSession() {
			t.Fatal("SSH not detected", key)
		}
		t.Setenv(key, "")
	}
}

func TestSSHClipboardGuidance(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "remote")
	for _, key := range []string{"WAYLAND_DISPLAY", "DISPLAY", "XDG_CURRENT_DESKTOP", "DBUS_SESSION_BUS_ADDRESS"} {
		t.Setenv(key, "")
	}
	_, err := DetectClipboard()
	if err == nil || !strings.Contains(err.Error(), "--mouse=false") || strings.Contains(err.Error(), "데스크톱 세션을 찾지") {
		t.Fatal(err)
	}
}
