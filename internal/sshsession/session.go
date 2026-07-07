// Package sshsession runs an interactive SSH session in a pseudo-terminal
// and renders its virtual screen as a lipgloss-styled string, so it can be
// embedded inside a bubbletea panel instead of suspending the whole
// program for it.
//
// This depends on two unexported implementation details of
// github.com/hinshun/vt10x (which has no tagged releases): the bit values
// of its internal text-attribute flags (attrReverse, attrBold — mirrored
// below since they aren't exported). If a future commit of that library
// changes those bit values, bold/reverse-video rendering could silently be
// wrong; character content and color (the load-bearing parts) don't depend
// on this and aren't affected.
package sshsession

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"

	"github.com/drkpkg/minissh/internal/connect"
	"github.com/drkpkg/minissh/internal/model"
)

// Mirrors vt10x's unexported attrReverse/attrBold bit values (state.go).
const (
	attrReverse = 1 << 0
	attrBold    = 1 << 2
)

// Session is one live SSH session running in a pty, parsed into a virtual
// terminal screen that Render paints into a bounded area on demand.
type Session struct {
	term vt10x.Terminal
	ptmx *os.File
	cmd  *exec.Cmd

	mu   sync.Mutex
	done chan struct{}
	err  error
}

// Start launches ssh for h in a new pty sized cols x rows.
func Start(h model.Host, cols, rows int) (*Session, error) {
	cmd, err := connect.Command(h)
	if err != nil {
		return nil, err
	}
	return startCmd(cmd, cols, rows)
}

// startCmd does the actual pty/vt10x wiring, factored out from Start so
// tests can run it against an arbitrary local command instead of requiring
// a real ssh binary and network access.
func startCmd(cmd *exec.Cmd, cols, rows int) (*Session, error) {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	term := vt10x.New(vt10x.WithSize(cols, rows))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, err
	}

	s := &Session{term: term, ptmx: ptmx, cmd: cmd, done: make(chan struct{})}
	go s.readLoop()
	return s, nil
}

func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			_, _ = s.term.Write(buf[:n])
		}
		if err != nil {
			s.mu.Lock()
			s.err = err
			s.mu.Unlock()
			close(s.done)
			return
		}
	}
}

// Write sends raw bytes (already translated from a key event) to the
// remote session.
func (s *Session) Write(p []byte) (int, error) {
	return s.ptmx.Write(p)
}

// Resize updates both the virtual terminal's screen size and the real
// pty's window size, so the remote shell's SIGWINCH-driven reflow matches
// what's actually being rendered.
func (s *Session) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	s.term.Resize(cols, rows)
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: clampU16(rows), Cols: clampU16(cols)})
}

func clampU16(v int) uint16 {
	if v < 0 {
		return 0
	}
	if v > 65535 {
		return 65535
	}
	return uint16(v)
}

// Done is closed once the session has ended — the remote closed the
// connection, or the local ssh process exited for any reason.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Err returns the reason the session ended, once Done is closed.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close releases the pty and, if it's still running, kills the ssh
// process.
func (s *Session) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.ptmx.Close()
}

// Render paints the current virtual screen as a lipgloss-styled string
// sized exactly width x height (blank-padded if the panel is taller than
// the pty), so it composes cleanly into a fixed-size panel.
func (s *Session) Render(width, height int) string {
	s.term.Lock()
	defer s.term.Unlock()

	cols, rows := s.term.Size()
	cx, cy := -1, -1
	if s.term.CursorVisible() {
		cur := s.term.Cursor()
		cx, cy = cur.X, cur.Y
	}

	var b strings.Builder
	for y := 0; y < height; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		if y >= rows {
			continue
		}
		b.WriteString(renderRow(s.term, y, min(width, cols), cx, cy))
	}
	return b.String()
}

// cellStyle is the plain-comparable subset of a glyph's rendering that
// actually varies cell to cell — used to batch consecutive same-styled
// cells into a single lipgloss.Render call instead of one per character.
type cellStyle struct {
	fg, bg vt10x.Color
	bold   bool
}

func glyphCellStyle(g vt10x.Glyph, isCursor bool) cellStyle {
	fg, bg := g.FG, g.BG
	if g.Mode&attrReverse != 0 || isCursor {
		fg, bg = bg, fg
	}
	return cellStyle{fg: fg, bg: bg, bold: g.Mode&attrBold != 0}
}

// lipglossStyle maps a vt10x color/attribute combination onto lipgloss.
// vt10x.Color's non-default values already are ANSI/256-palette indices,
// which is exactly what lipgloss.Color("N") (a numeric string) means too.
func (cs cellStyle) lipglossStyle() lipgloss.Style {
	style := lipgloss.NewStyle()
	if cs.fg != vt10x.DefaultFG {
		style = style.Foreground(lipgloss.Color(strconv.Itoa(int(cs.fg))))
	}
	if cs.bg != vt10x.DefaultBG {
		style = style.Background(lipgloss.Color(strconv.Itoa(int(cs.bg))))
	}
	if cs.bold {
		style = style.Bold(true)
	}
	return style
}

func renderRow(term vt10x.View, y, width, cx, cy int) string {
	var b strings.Builder
	var run strings.Builder
	var runStyle cellStyle
	haveRun := false

	flush := func() {
		if haveRun && run.Len() > 0 {
			b.WriteString(runStyle.lipglossStyle().Render(run.String()))
		}
		run.Reset()
		haveRun = false
	}

	for x := 0; x < width; x++ {
		g := term.Cell(x, y)
		st := glyphCellStyle(g, x == cx && y == cy)
		if !haveRun {
			runStyle = st
			haveRun = true
		} else if st != runStyle {
			flush()
			runStyle = st
			haveRun = true
		}
		ch := g.Char
		if ch == 0 {
			ch = ' '
		}
		run.WriteRune(ch)
	}
	flush()
	return b.String()
}
