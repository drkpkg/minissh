package sshsession

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hinshun/vt10x"
	"github.com/muesli/termenv"
)

// TestMain forces a color-capable profile: lipgloss's default renderer
// auto-detects color support from os.Stdout, which isn't a real terminal
// under `go test` — without this, every Render() call would silently
// produce plain, uncolored text and mask real coloring bugs.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

// --- pure rendering logic, no pty/terminal needed --------------------------

func TestGlyphCellStyleAppliesReverseVideo(t *testing.T) {
	g := vt10x.Glyph{FG: 1, BG: 2, Mode: attrReverse}
	cs := glyphCellStyle(g, false)
	if cs.fg != 2 || cs.bg != 1 {
		t.Fatalf("expected fg/bg swapped under reverse video, got fg=%d bg=%d", cs.fg, cs.bg)
	}
}

func TestGlyphCellStyleCursorAlsoSwapsColors(t *testing.T) {
	g := vt10x.Glyph{FG: 1, BG: 2}
	cs := glyphCellStyle(g, true)
	if cs.fg != 2 || cs.bg != 1 {
		t.Fatalf("expected fg/bg swapped at the cursor cell, got fg=%d bg=%d", cs.fg, cs.bg)
	}
}

func TestGlyphCellStyleBoldFlag(t *testing.T) {
	g := vt10x.Glyph{FG: vt10x.DefaultFG, BG: vt10x.DefaultBG, Mode: attrBold}
	if !glyphCellStyle(g, false).bold {
		t.Fatal("expected bold flag set")
	}
	g.Mode = 0
	if glyphCellStyle(g, false).bold {
		t.Fatal("expected bold flag unset")
	}
}

func TestCellStyleLipglossStyleOmitsDefaultColors(t *testing.T) {
	cs := cellStyle{fg: vt10x.DefaultFG, bg: vt10x.DefaultBG}
	rendered := cs.lipglossStyle().Render("x")
	if rendered != "x" {
		t.Fatalf("expected no ANSI wrapping for default colors, got %q", rendered)
	}
}

func TestCellStyleLipglossStyleAppliesExplicitColor(t *testing.T) {
	cs := cellStyle{fg: vt10x.Red, bg: vt10x.DefaultBG}
	style := cs.lipglossStyle()
	// Assert on the style's own state (renderer-independent) rather than
	// on rendered ANSI bytes, which depend on lipgloss's terminal
	// auto-detection.
	if got := style.GetForeground(); got != lipgloss.Color("1") {
		t.Fatalf("expected foreground color '1' (vt10x.Red), got %v", got)
	}

	rendered := style.Render("x")
	if !strings.Contains(rendered, "x") {
		t.Fatalf("expected rendered output to still contain the character, got %q", rendered)
	}
}

// --- rendering against a real vt10x.Terminal, no pty needed -----------------

func TestRenderShowsPlainText(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	term.Write([]byte("hello"))

	s := &Session{term: term}
	out := s.Render(10, 2)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "hello") {
		t.Fatalf("expected first line to contain 'hello', got %q", lines[0])
	}
}

func TestRenderAppliesANSIColor(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 1))
	term.Write([]byte("\x1b[31mred\x1b[0m"))

	s := &Session{term: term}
	out := s.Render(10, 1)
	if !strings.Contains(out, "red") {
		t.Fatalf("expected rendered text to contain 'red', got %q", out)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected some ANSI escape in colored output, got %q", out)
	}
}

func TestRenderPadsShorterThanRequestedHeight(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	s := &Session{term: term}
	out := s.Render(10, 5) // taller than the pty's 2 rows
	if got := strings.Count(out, "\n"); got != 4 {
		t.Fatalf("expected 5 lines (4 newlines), got %d newlines in %q", got, out)
	}
}

// --- real pty lifecycle, using a harmless local command instead of ssh -----

func TestStartCmdRunsCommandAndCapturesOutput(t *testing.T) {
	cmd := exec.Command("sh", "-c", "printf hello")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer s.Close()

	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the command to finish")
	}

	out := s.Render(20, 5)
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected rendered output to contain 'hello', got %q", out)
	}
}

func TestStartCmdWriteSendsInputToProcess(t *testing.T) {
	// cat echoes stdin back through the pty, proving Write() actually
	// reaches the child process rather than being a no-op.
	cmd := exec.Command("cat")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer s.Close()

	if _, err := s.Write([]byte("ping\r")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.Render(20, 5), "ping") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected echoed input 'ping' to appear in the rendered output, got %q", s.Render(20, 5))
}

func TestStartCmdResizeUpdatesTerminalSize(t *testing.T) {
	cmd := exec.Command("cat")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer s.Close()

	if err := s.Resize(40, 10); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	cols, rows := s.term.Size()
	if cols != 40 || rows != 10 {
		t.Fatalf("expected terminal resized to 40x10, got %dx%d", cols, rows)
	}
}

func TestStartCmdCloseKillsRunningProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("expected Done() to close after Close() kills the process")
	}
}

func TestStartCmdDefaultsInvalidSize(t *testing.T) {
	cmd := exec.Command("true")
	s, err := startCmd(cmd, 0, -5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer s.Close()

	cols, rows := s.term.Size()
	if cols != 80 || rows != 24 {
		t.Fatalf("expected default 80x24, got %dx%d", cols, rows)
	}
}
