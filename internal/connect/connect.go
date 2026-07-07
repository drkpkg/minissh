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

	"github.com/drkpkg/minissh/internal/keychain"
	"github.com/drkpkg/minissh/internal/model"
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
// injects it non-interactively. Key-auth hosts get the same treatment for
// a stored key passphrase. Otherwise it falls back to plain `ssh`, which
// prompts interactively itself — this is what the CSV/JSON importers hit
// before keychain-storage of a secret has happened, and what any host hits
// if sshpass isn't installed.
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
	switch h.Identity.Kind {
	case model.IdentityPassword:
		if pw, err := keychain.GetPassword(h.ID); err == nil && pw != "" {
			if bin, argv, ok := sshpassCommand(pw, "", h); ok {
				return bin, argv, nil
			}
		}
	case model.IdentityKey:
		// sshpass's default prompt detection looks for "assword", which
		// doesn't match ssh's "Enter passphrase for key ...:" prompt — -P
		// overrides it with a substring that does.
		if pass, err := keychain.GetKeyPassphrase(h.ID); err == nil && pass != "" {
			if bin, argv, ok := sshpassCommand(pass, "passphrase", h); ok {
				return bin, argv, nil
			}
		}
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return "", nil, fmt.Errorf("ssh binary not found in PATH: %w", err)
	}
	return sshBin, append([]string{sshBin}, Args(h)...), nil
}

// sshpassCommand builds an argv that runs ssh for h under sshpass, feeding
// it secret non-interactively. prompt, if non-empty, is passed as sshpass's
// -P (a substring to match in the prompt instead of its "assword" default).
// ok is false if either binary isn't on PATH, so the caller can fall back
// to plain ssh (which will just prompt interactively) instead of erroring.
func sshpassCommand(secret, prompt string, h model.Host) (bin string, argv []string, ok bool) {
	sshpassBin, err := exec.LookPath("sshpass")
	if err != nil {
		return "", nil, false
	}
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return "", nil, false
	}
	head := []string{sshpassBin}
	if prompt != "" {
		head = append(head, "-P", prompt)
	}
	head = append(head, "-p", secret, sshBin)
	return sshpassBin, append(head, Args(h)...), true
}
