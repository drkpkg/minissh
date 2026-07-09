package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/drkpkg/minissh/internal/sshsession"
)

// selPoint is a session-local column/row coordinate, the same space
// Session.Render/Session.SelectedText use.
type selPoint struct{ X, Y int }

// copyToClipboard is termenv.Copy, substitutable in tests.
var copyToClipboard = termenv.Copy

// sessionPanelOrigin returns the screen-space (x, y) of the top-left cell
// of the active embedded session's rendered content — i.e. where
// session-local column/row (0,0) lands — for translating mouse events into
// session coordinates. Derived fresh from existing layout fields each call
// rather than cached from a render pass: View() has a value receiver
// (bubbletea's contract), so anything it computed couldn't survive back
// into the next Update() anyway. Mirrors the geometry mainView() actually
// builds (see the panel composition there): groupsBox contributes its
// content width plus its own 2 border columns, hostsBox adds 1 more for
// its own left border; vertically, the optional session tab strip and
// search bar each add a line above the panels, then the hostsBox top
// border, the "SESSION — label" header, and (if the session died) the
// ended banner.
func (m appModel) sessionPanelOrigin() (x, y int) {
	x = m.groupsWidth + 3

	y = 1 // hostsBox top border
	if len(m.sessions) > 0 {
		y++ // session tab strip
	}
	if m.searching {
		y++ // search bar
	}
	y++ // "SESSION — label" header line
	if m.currentSessionIdx < len(m.sessions) && m.sessions[m.currentSessionIdx].ended {
		y++ // "Session ended..." banner
	}
	return x, y
}

// sessionContentSize returns the width/height Session.Render was actually
// last called with for the current session, so mouse hit-testing and
// SelectedText agree on the same bounds Render used.
func (m appModel) sessionContentSize(ended bool) (width, height int) {
	height = m.hostsContentHeight
	if ended {
		height--
	}
	return m.hostsWidth, height
}

// currentSelection returns the active session's selection normalized into
// a *sshsession.Selection ready for Render, or nil if there isn't one (or
// it belongs to a different session than the one currently focused).
func (m appModel) currentSelection() *sshsession.Selection {
	if m.selSessionID == "" || m.currentSessionIdx >= len(m.sessions) {
		return nil
	}
	cur := m.sessions[m.currentSessionIdx]
	if cur.host.ID != m.selSessionID {
		return nil
	}
	a, h := m.selAnchor, m.selHead
	if a.Y > h.Y || (a.Y == h.Y && a.X > h.X) {
		a, h = h, a
	}
	if a == h {
		return nil // a bare click with no drag selects nothing
	}
	return &sshsession.Selection{StartX: a.X, StartY: a.Y, EndX: h.X, EndY: h.Y}
}

// clearSelection drops any in-progress or completed selection — called
// whenever it would otherwise go stale: a keypress, switching sessions, or
// starting a new drag.
func (m *appModel) clearSelection() {
	m.selecting = false
	m.selSessionID = ""
	m.selAnchor, m.selHead = selPoint{}, selPoint{}
}

// updateSessionMouse handles a mouse event while a session is focused,
// scoped to the session panel's on-screen bounds — outside that region (or
// anywhere else in the app, since nothing else handles mouse input) clicks
// are simply ignored, leaving native OS/terminal selection (via
// shift+drag, the standard bypass every major terminal emulator supports)
// as the only way to select text elsewhere.
func (m appModel) updateSessionMouse(mm tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.currentSessionIdx >= len(m.sessions) {
		return m, nil
	}
	cur := m.sessions[m.currentSessionIdx]
	ox, oy := m.sessionPanelOrigin()
	width, height := m.sessionContentSize(cur.ended)

	px, py := mm.X-ox, mm.Y-oy
	inBounds := px >= 0 && px < width && py >= 0 && py < height
	if px < 0 {
		px = 0
	} else if px >= width {
		px = width - 1
	}
	if py < 0 {
		py = 0
	} else if py >= height {
		py = height - 1
	}
	pt := selPoint{X: px, Y: py}

	switch {
	case mm.Action == tea.MouseActionPress && mm.Button == tea.MouseButtonLeft:
		if !inBounds {
			return m, nil
		}
		m.selecting = true
		m.selSessionID = cur.host.ID
		m.selAnchor, m.selHead = pt, pt
		return m, nil
	case mm.Action == tea.MouseActionMotion && m.selecting && m.selSessionID == cur.host.ID:
		m.selHead = pt
		return m, nil
	case mm.Action == tea.MouseActionRelease && m.selecting && m.selSessionID == cur.host.ID:
		m.selecting = false
		m.selHead = pt
		sel := m.currentSelection()
		if sel == nil {
			return m, nil
		}
		text := cur.sess.SelectedText(*sel, width, height)
		if text == "" {
			return m, nil
		}
		return m, func() tea.Msg {
			copyToClipboard(text)
			return nil
		}
	}
	return m, nil
}
