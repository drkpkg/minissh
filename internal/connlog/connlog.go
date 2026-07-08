// Package connlog appends a plain-text line per SSH connection attempt to a
// log file under the config directory, so a failure that flashes by on
// screen (or happens while nobody's watching) can still be reviewed
// afterward — no tooling required to read it, just `cat`/`tail`.
package connlog

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/drkpkg/minissh/internal/model"
)

// Outcome is one connection attempt's result.
type Outcome struct {
	Host     model.Host
	Mode     string // "embedded" or "fullscreen"
	Started  time.Time
	Duration time.Duration
	ExitCode int   // 0 on success; -1 if unknown (e.g. killed by a signal)
	Err      error // non-nil description of what went wrong, if any
}

// Path returns the location of the connection log, honoring
// XDG_CONFIG_HOME.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "minissh", "connections.log"), nil
}

// Append writes one line describing o to the log, creating the file (and
// its directory) if needed.
func Append(o Outcome) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(formatLine(o) + "\n")
	return err
}

func formatLine(o Outcome) string {
	status := "ok"
	if o.ExitCode != 0 || o.Err != nil {
		status = "failed"
	}
	target := fmt.Sprintf("%s (%s)", o.Host.Label, o.Host.Address)
	line := fmt.Sprintf("%s host=%q mode=%s result=%s duration=%s",
		o.Started.Format(time.RFC3339), target, o.Mode, status, o.Duration.Round(time.Millisecond))
	if o.ExitCode != 0 {
		line += fmt.Sprintf(" exit=%d", o.ExitCode)
	}
	if o.Err != nil {
		line += fmt.Sprintf(" error=%q", o.Err.Error())
	}
	return line
}
