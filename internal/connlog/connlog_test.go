package connlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drkpkg/minissh/internal/model"
)

func TestPathHonorsXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join(dir, "minissh", "connections.log")
	if path != want {
		t.Fatalf("Path() = %q, want %q", path, want)
	}
}

func TestAppendCreatesFileAndDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	h := model.Host{Label: "alpha", Address: "10.0.0.1"}
	if err := Append(Outcome{Host: h, Mode: "embedded", Started: time.Now(), Duration: 45 * time.Millisecond}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	path, _ := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "alpha (10.0.0.1)") {
		t.Fatalf("expected log line to mention the host, got %q", string(data))
	}
	if !strings.Contains(string(data), "result=ok") {
		t.Fatalf("expected result=ok for a zero-exit-code outcome, got %q", string(data))
	}
}

func TestAppendMarksNonZeroExitAsFailed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	h := model.Host{Label: "beta", Address: "10.0.0.2"}
	err := Append(Outcome{
		Host: h, Mode: "embedded", Started: time.Now(), Duration: 200 * time.Millisecond,
		ExitCode: 255, Err: errors.New("Connection refused"),
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	path, _ := Path()
	data, _ := os.ReadFile(path)
	line := string(data)
	if !strings.Contains(line, "result=failed") {
		t.Fatalf("expected result=failed, got %q", line)
	}
	if !strings.Contains(line, "exit=255") {
		t.Fatalf("expected exit=255, got %q", line)
	}
	if !strings.Contains(line, `error="Connection refused"`) {
		t.Fatalf("expected quoted error text, got %q", line)
	}
}

func TestAppendAccumulatesMultipleLines(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	h := model.Host{Label: "gamma", Address: "10.0.0.3"}
	for i := 0; i < 3; i++ {
		if err := Append(Outcome{Host: h, Mode: "embedded", Started: time.Now()}); err != nil {
			t.Fatalf("Append #%d: %v", i, err)
		}
	}

	path, _ := Path()
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 accumulated lines, got %d: %q", len(lines), string(data))
	}
}
