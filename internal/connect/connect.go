// Package connect turns a Host into a real interactive SSH session by
// exec-replacing the current process with the system ssh binary — this
// reuses ~/.ssh/config, agent forwarding, host key checking, and terminal
// handling exactly as plain `ssh` would.
package connect

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/danieluremix/minissh/internal/keychain"
	"github.com/danieluremix/minissh/internal/model"
)

// Args builds the argument vector (excluding argv[0]) to pass to the ssh
// binary for the given host. Exported separately from Exec so it's testable
// without actually spawning a process.
func Args(h model.Host) []string {
	var args []string

	if h.Identity.Kind == model.IdentityKey && h.Identity.KeyPath != "" {
		args = append(args, "-i", h.Identity.KeyPath)
	}
	if h.Port != 0 && h.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", h.Port))
	}

	dest := h.Address
	if h.Username != "" {
		dest = h.Username + "@" + h.Address
	}
	args = append(args, dest)

	return args
}

// Exec replaces the current process with an ssh session for h. On success
// it never returns; the calling process becomes the ssh session. This is
// for direct CLI passthrough use (`minissh connect <label>`), where
// replacing the process is the correct, expected behavior — exit codes and
// signal handling propagate exactly as if the user had run `ssh` directly.
//
// For password-auth hosts, it looks up a stored password in the OS
// keychain and, if both a password and the `sshpass` binary are available,
// injects it non-interactively. Otherwise it falls back to plain `ssh`,
// which prompts for the password itself — this is what the CSV/JSON
// importers hit before keychain-storage of a password has happened, and
// what any host hits if sshpass isn't installed.
func Exec(h model.Host) error {
	bin, argv, err := buildCommand(h)
	if err != nil {
		return err
	}
	return syscall.Exec(bin, argv, os.Environ())
}

// Command builds an *exec.Cmd for h without running it, for callers that
// want ssh as a genuine child process rather than a process replacement —
// e.g. the TUI, which stays running across SSH sessions via
// tea.ExecProcess. Stdin/Stdout/Stderr are deliberately left unset: bubble
// tea's ExecProcess wires those up itself (to the terminal it just
// released) before running the command.
func Command(h model.Host) (*exec.Cmd, error) {
	bin, argv, err := buildCommand(h)
	if err != nil {
		return nil, err
	}
	return exec.Command(bin, argv[1:]...), nil
}

func buildCommand(h model.Host) (bin string, argv []string, err error) {
	if h.Identity.Kind == model.IdentityPassword {
		if pw, err := keychain.GetPassword(h.ID); err == nil && pw != "" {
			if sshpassBin, err := exec.LookPath("sshpass"); err == nil {
				sshBin, err := exec.LookPath("ssh")
				if err != nil {
					return "", nil, fmt.Errorf("ssh binary not found in PATH: %w", err)
				}
				argv := append([]string{sshpassBin, "-p", pw, sshBin}, Args(h)...)
				return sshpassBin, argv, nil
			}
		}
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return "", nil, fmt.Errorf("ssh binary not found in PATH: %w", err)
	}
	return sshBin, append([]string{sshBin}, Args(h)...), nil
}
