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
	// fg/bg are left untouched — reverse video is applied by the terminal
	// via the reverse flag, not by swapping (possibly sentinel) color
	// values ourselves. See the cellStyle doc comment for why.
	if cs.fg != 1 || cs.bg != 2 || !cs.reverse {
		t.Fatalf("expected fg=1 bg=2 reverse=true, got fg=%d bg=%d reverse=%v", cs.fg, cs.bg, cs.reverse)
	}
}

func TestGlyphCellStyleCursorAlsoSetsReverse(t *testing.T) {
	g := vt10x.Glyph{FG: 1, BG: 2}
	cs := glyphCellStyle(g, true)
	if cs.fg != 1 || cs.bg != 2 || !cs.reverse {
		t.Fatalf("expected fg=1 bg=2 reverse=true at the cursor cell, got fg=%d bg=%d reverse=%v", cs.fg, cs.bg, cs.reverse)
	}
}

func TestGlyphCellStyleCursorOnReverseCellCancelsOut(t *testing.T) {
	g := vt10x.Glyph{FG: 1, BG: 2, Mode: attrReverse}
	cs := glyphCellStyle(g, true)
	if cs.reverse {
		t.Fatalf("expected cursor on an already-reverse cell to XOR back to non-reverse, got reverse=%v", cs.reverse)
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

// TestCellStyleLipglossStyleCursorOnDefaultColorsStaysVisible guards
// against the actual bug reported in production: the cursor cell over
// ordinary (default-colored) text rendered as invisible, because the old
// implementation swapped vt10x.DefaultFG/DefaultBG — sentinels, not real
// palette indices — into literal lipgloss.Color numbers, producing a
// bogus 256-color escape sequence most terminals silently drop.
func TestCellStyleLipglossStyleCursorOnDefaultColorsStaysVisible(t *testing.T) {
	g := vt10x.Glyph{FG: vt10x.DefaultFG, BG: vt10x.DefaultBG}
	cs := glyphCellStyle(g, true)
	style := cs.lipglossStyle()
	if !style.GetReverse() {
		t.Fatal("expected the cursor cell to use the terminal's own reverse-video attribute")
	}
	if got := style.GetForeground(); got != lipgloss.TerminalColor(lipgloss.NoColor{}) {
		t.Fatalf("expected no literal foreground override for default-colored cursor cell, got %v", got)
	}
	if got := style.GetBackground(); got != lipgloss.TerminalColor(lipgloss.NoColor{}) {
		t.Fatalf("expected no literal background override for default-colored cursor cell, got %v", got)
	}
}

func TestBlinkOnAtTogglesEveryInterval(t *testing.T) {
	base := time.UnixMilli(0)
	if !blinkOnAt(base) {
		t.Fatal("expected blink on at the start of a cycle")
	}
	if blinkOnAt(base.Add(cursorBlinkInterval)) {
		t.Fatal("expected blink off one interval later")
	}
	if !blinkOnAt(base.Add(2 * cursorBlinkInterval)) {
		t.Fatal("expected blink on again two intervals later")
	}
	if !blinkOnAt(base.Add(cursorBlinkInterval / 2)) {
		t.Fatal("expected still on partway through the first interval")
	}
}

// --- rendering against a real vt10x.Terminal, no pty needed -----------------

func TestRenderShowsPlainText(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	_, _ = term.Write([]byte("hello"))

	s := &Session{term: term}
	out := s.Render(10, 2, nil)
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
	_, _ = term.Write([]byte("\x1b[31mred\x1b[0m"))

	s := &Session{term: term}
	out := s.Render(10, 1, nil)
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
	out := s.Render(10, 5, nil) // taller than the pty's 2 rows
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
	defer func() { _ = s.Close() }()

	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the command to finish")
	}

	out := s.Render(20, 5, nil)
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
	defer func() { _ = s.Close() }()

	if _, err := s.Write([]byte("ping\r")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.Render(20, 5, nil), "ping") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected echoed input 'ping' to appear in the rendered output, got %q", s.Render(20, 5, nil))
}

func TestStartCmdResizeUpdatesTerminalSize(t *testing.T) {
	cmd := exec.Command("cat")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer func() { _ = s.Close() }()

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

func TestStartCmdReapsProcessAndCapturesExitCode(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 7")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer func() { _ = s.Close() }()

	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the command to finish")
	}

	if cmd.ProcessState == nil {
		t.Fatal("expected cmd.Wait() to have been called (ProcessState set) — otherwise the process is left a zombie")
	}
	res, ok := s.Result()
	if !ok {
		t.Fatal("expected Result() to be ready once Done is closed")
	}
	if res.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", res.ExitCode)
	}
	if res.ClosedByUser {
		t.Fatal("expected ClosedByUser false — this session ended on its own, not via Close")
	}
	if res.Duration <= 0 {
		t.Fatalf("expected a positive duration, got %v", res.Duration)
	}
}

func TestResultNotOkBeforeSessionEnds(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer func() { _ = s.Close() }()

	if _, ok := s.Result(); ok {
		t.Fatal("expected Result() not ok while the session is still running")
	}
}

func TestClosedByUserReflectsExplicitClose(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	s, err := startCmd(cmd, 20, 5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	if s.ClosedByUser() {
		t.Fatal("expected ClosedByUser false before Close is called")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !s.ClosedByUser() {
		t.Fatal("expected ClosedByUser true immediately after Close, even before Done fires")
	}

	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Done")
	}
	res, ok := s.Result()
	if !ok || !res.ClosedByUser {
		t.Fatalf("expected Result().ClosedByUser true, got ok=%v res=%+v", ok, res)
	}
}

// --- selection ----------------------------------------------------------

func TestSelectedTextSingleRow(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 1))
	_, _ = term.Write([]byte("hello"))
	s := &Session{term: term}

	// "hello" occupies columns 0-4; select columns 1-3 ("ell").
	sel := Selection{StartX: 1, StartY: 0, EndX: 3, EndY: 0}
	got := s.SelectedText(sel, 10, 1)
	if got != "ell" {
		t.Fatalf("SelectedText = %q, want %q", got, "ell")
	}
}

func TestSelectedTextMultiRowTrimsTrailingSpaces(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	_, _ = term.Write([]byte("one\r\ntwo"))
	s := &Session{term: term}

	// From column 1 of row 0 through column 1 of row 1: "ne" then "tw".
	sel := Selection{StartX: 1, StartY: 0, EndX: 1, EndY: 1}
	got := s.SelectedText(sel, 10, 2)
	if got != "ne\ntw" {
		t.Fatalf("SelectedText = %q, want %q", got, "ne\ntw")
	}
}

func TestSelectedTextFromScrolledBackView(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	_, _ = term.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\n"))
	s := &Session{term: term}

	s.ScrollUp(2) // view now shows history rows "two", "three" (see TestScrollUpRevealsHistoryAboveLiveScreen)
	sel := Selection{StartX: 0, StartY: 0, EndX: 2, EndY: 0}
	got := s.SelectedText(sel, 10, 2)
	if got != "two" {
		t.Fatalf("SelectedText from a scrolled-back view = %q, want %q", got, "two")
	}
}

func TestRenderHighlightsSelectionAsReverseVideo(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 1))
	_, _ = term.Write([]byte("hello"))
	s := &Session{term: term}

	plain := s.Render(10, 1, nil)
	sel := &Selection{StartX: 0, StartY: 0, EndX: 4, EndY: 0}
	highlighted := s.Render(10, 1, sel)
	if plain == highlighted {
		t.Fatal("expected a selection to change the rendered output (reverse video)")
	}
	if !strings.Contains(highlighted, "\x1b[") {
		t.Fatalf("expected an ANSI escape in the selection-highlighted output, got %q", highlighted)
	}
}

func TestSelectionRowRangeClampsToRowBounds(t *testing.T) {
	sel := Selection{StartX: 5, StartY: 0, EndX: 2, EndY: 2}
	if _, _, ok := sel.rowRange(-1, 10); ok {
		t.Fatal("expected no range above the selection's start row")
	}
	if _, _, ok := sel.rowRange(3, 10); ok {
		t.Fatal("expected no range below the selection's end row")
	}
	if x0, x1, ok := sel.rowRange(0, 10); !ok || x0 != 5 || x1 != 10 {
		t.Fatalf("start row range = (%d, %d, %v), want (5, 10, true)", x0, x1, ok)
	}
	if x0, x1, ok := sel.rowRange(1, 10); !ok || x0 != 0 || x1 != 10 {
		t.Fatalf("middle row range = (%d, %d, %v), want (0, 10, true)", x0, x1, ok)
	}
	if x0, x1, ok := sel.rowRange(2, 10); !ok || x0 != 0 || x1 != 3 {
		t.Fatalf("end row range = (%d, %d, %v), want (0, 3, true)", x0, x1, ok)
	}
}

// --- scrollback -------------------------------------------------------

func TestScrollUpRevealsHistoryAboveLiveScreen(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	s := &Session{term: term}

	// Print more lines than fit on screen so the earliest ones scroll into
	// history; \r\n so each is its own row regardless of wrap behavior.
	_, _ = term.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\n"))

	// "one"/"two"/"three" scroll into history in that order, leaving
	// "four" (and a trailing blank row) live.
	if got := s.term.HistoryLen(); got != 3 {
		t.Fatalf("expected 3 rows scrolled into history, got %d", got)
	}

	s.ScrollUp(2)
	out := s.Render(10, 2, nil)
	if strings.Contains(out, "four") {
		t.Fatalf("expected scrolling up 2 rows to move past the live line entirely, got %q", out)
	}
	if !strings.Contains(out, "two") || !strings.Contains(out, "three") {
		t.Fatalf("expected the view scrolled up 2 rows to show the two preceding history lines, got %q", out)
	}
}

func TestScrollDownReturnsTowardLiveOutput(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	s := &Session{term: term}
	_, _ = term.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\n"))

	s.ScrollUp(100) // clamps to however much history actually exists
	if s.ScrollOffset() == 0 {
		t.Fatal("expected ScrollUp to move off live output")
	}

	s.ScrollDown(100) // clamps back to 0
	if s.ScrollOffset() != 0 {
		t.Fatalf("expected ScrollDown to clamp back to live output, got offset %d", s.ScrollOffset())
	}
	out := s.Render(10, 2, nil)
	if !strings.Contains(out, "four") {
		t.Fatalf("expected live view restored after scrolling back down, got %q", out)
	}
}

func TestScrollResetSnapsToLive(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	s := &Session{term: term}
	_, _ = term.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\n"))

	s.ScrollUp(1)
	if s.ScrollOffset() == 0 {
		t.Fatal("expected ScrollUp to move off live output")
	}
	s.ScrollReset()
	if s.ScrollOffset() != 0 {
		t.Fatalf("expected ScrollReset to snap back to 0, got %d", s.ScrollOffset())
	}
}

func TestInAltScreenReflectsAlternateBuffer(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	s := &Session{term: term}

	if s.InAltScreen() {
		t.Fatal("expected primary screen by default")
	}

	// DECSET 1049: switch to the alternate screen buffer, as full-screen
	// programs like less/vim/top do on startup.
	_, _ = term.Write([]byte("\x1b[?1049h"))
	if !s.InAltScreen() {
		t.Fatal("expected alt screen after DECSET 1049")
	}

	_, _ = term.Write([]byte("\x1b[?1049l"))
	if s.InAltScreen() {
		t.Fatal("expected primary screen after leaving the alt screen")
	}
}

func TestAltScreenScrollingIsNotCapturedAsHistory(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(10, 2))
	_, _ = term.Write([]byte("\x1b[?1049h")) // enter alt screen, like less/vim/top
	_, _ = term.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\n"))

	if got := term.HistoryLen(); got != 0 {
		t.Fatalf("expected no scrollback captured while on the alt screen, got %d rows", got)
	}
}

func TestStartCmdDefaultsInvalidSize(t *testing.T) {
	cmd := exec.Command("true")
	s, err := startCmd(cmd, 0, -5)
	if err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	defer func() { _ = s.Close() }()

	cols, rows := s.term.Size()
	if cols != 80 || rows != 24 {
		t.Fatalf("expected default 80x24, got %dx%d", cols, rows)
	}
}
