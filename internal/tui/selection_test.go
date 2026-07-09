package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/drkpkg/minissh/internal/model"
)

func TestSessionPanelOriginAccountsForTabStripSearchAndBanner(t *testing.T) {
	m := appModel{groupsWidth: 20}
	x, y := m.sessionPanelOrigin()
	if x != 23 { // groupsWidth(20) + groupsBox border(2) + hostsBox left border(1)
		t.Fatalf("x = %d, want 23", x)
	}
	if y != 2 { // hostsBox top border(1) + "SESSION — label" header(1)
		t.Fatalf("y with no tab strip/search/banner = %d, want 2", y)
	}

	m.sessions = []liveSession{{}}
	if _, y := m.sessionPanelOrigin(); y != 3 {
		t.Fatalf("y with the session tab strip visible = %d, want 3", y)
	}

	m.searching = true
	if _, y := m.sessionPanelOrigin(); y != 4 {
		t.Fatalf("y with the search bar also visible = %d, want 4", y)
	}

	m.searching = false
	m.sessions[0].ended = true
	if _, y := m.sessionPanelOrigin(); y != 4 {
		t.Fatalf("y with the ended banner instead of the search bar = %d, want 4", y)
	}
}

func TestCurrentSelectionNormalizesAndRequiresADrag(t *testing.T) {
	m := appModel{
		sessions:          []liveSession{{host: model.Host{ID: "h1"}}},
		currentSessionIdx: 0,
	}

	if m.currentSelection() != nil {
		t.Fatal("expected no selection before one is made")
	}

	m.selSessionID = "h1"
	m.selAnchor = selPoint{X: 3, Y: 1}
	m.selHead = selPoint{X: 3, Y: 1}
	if m.currentSelection() != nil {
		t.Fatal("expected a bare click (anchor == head) to select nothing")
	}

	// Dragging "backward" (head before anchor in reading order) must still
	// normalize into a Start-before-End selection.
	m.selAnchor = selPoint{X: 5, Y: 2}
	m.selHead = selPoint{X: 1, Y: 0}
	sel := m.currentSelection()
	if sel == nil {
		t.Fatal("expected a selection after a drag")
	}
	if sel.StartX != 1 || sel.StartY != 0 || sel.EndX != 5 || sel.EndY != 2 {
		t.Fatalf("expected normalized selection (1,0)-(5,2), got (%d,%d)-(%d,%d)",
			sel.StartX, sel.StartY, sel.EndX, sel.EndY)
	}

	m.selSessionID = "some-other-session"
	if m.currentSelection() != nil {
		t.Fatal("expected a selection tagged to a different session to be ignored")
	}
}

func TestMouseDragTracksPressMotionRelease(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	ox, oy := mm.sessionPanelOrigin()

	updated, _ = mm.updateActiveSession(tea.MouseMsg{X: ox + 2, Y: oy, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	mm = updated.(appModel)
	if !mm.selecting {
		t.Fatal("expected a left-button press inside the session panel to start a selection")
	}
	if mm.selAnchor != (selPoint{X: 2, Y: 0}) {
		t.Fatalf("expected anchor at session-local (2,0), got %+v", mm.selAnchor)
	}
	if mm.selSessionID != h.ID {
		t.Fatalf("expected the selection tagged to %q, got %q", h.ID, mm.selSessionID)
	}

	updated, _ = mm.updateActiveSession(tea.MouseMsg{X: ox + 5, Y: oy, Action: tea.MouseActionMotion})
	mm = updated.(appModel)
	if !mm.selecting {
		t.Fatal("expected motion mid-drag to keep selecting true")
	}
	if mm.selHead != (selPoint{X: 5, Y: 0}) {
		t.Fatalf("expected head to follow the drag to session-local (5,0), got %+v", mm.selHead)
	}

	updated, _ = mm.updateActiveSession(tea.MouseMsg{X: ox + 5, Y: oy, Action: tea.MouseActionRelease})
	mm = updated.(appModel)
	if mm.selecting {
		t.Fatal("expected release to end the drag")
	}
	if sel := mm.currentSelection(); sel == nil || sel.StartX != 2 || sel.EndX != 5 {
		t.Fatalf("expected the completed selection to span session-local columns 2-5, got %+v", sel)
	}
}

// TestMouseClickWithoutDragCopiesNothing is deterministic regardless of
// whatever the (still-connecting) remote session has actually printed:
// currentSelection already returns nil for anchor == head before
// SelectedText is ever consulted, so a bare click must never return a copy
// command.
func TestMouseClickWithoutDragCopiesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	ox, oy := mm.sessionPanelOrigin()

	updated, _ = mm.updateActiveSession(tea.MouseMsg{X: ox + 1, Y: oy, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	mm = updated.(appModel)

	updated, cmd := mm.updateActiveSession(tea.MouseMsg{X: ox + 1, Y: oy, Action: tea.MouseActionRelease})
	mm = updated.(appModel)
	if cmd != nil {
		t.Fatal("expected a bare click (no drag) to return no copy command")
	}
	if mm.currentSelection() != nil {
		t.Fatal("expected a bare click to leave no selection behind")
	}
}

func TestMouseClickOutsideSessionPanelIsIgnored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	updated, _ = mm.updateActiveSession(tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	mm = updated.(appModel)
	if mm.selecting {
		t.Fatal("expected a click outside the session panel (e.g. the groups panel) not to start a selection")
	}
}

func TestKeypressClearsAnInProgressSelection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	mm.selecting = true
	mm.selSessionID = h.ID
	mm.selAnchor = selPoint{X: 0, Y: 0}
	mm.selHead = selPoint{X: 3, Y: 0}

	updated, _ = mm.updateActiveSession(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	mm = updated.(appModel)
	if mm.selecting || mm.currentSelection() != nil {
		t.Fatal("expected any keypress to dismiss a stale selection")
	}
}
