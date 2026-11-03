package garden

import "os"

// SSHSession detects an SSH shell, including sessions without a desktop display.
func SSHSession() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""
}
