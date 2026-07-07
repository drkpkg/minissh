package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/drkpkg/minissh/internal/model"
	"github.com/drkpkg/minissh/internal/store"
)

// testHostForSession is a syntactically valid but guaranteed-non-routable
// address (RFC 5737 TEST-NET-1, same technique already used in
// internal/status's tests) — starting a real ssh process against it is
// fast (no DNS lookup, fails at the routing layer) and never actually
// reaches a network peer. These tests are about pty/session *lifecycle*
// wiring, not about a real SSH connection succeeding.
func testHostForSession() model.Host {
	return model.Host{ID: "h1", Label: "test-host", Address: "192.0.2.1", Port: 22}
}

func testHostForSession2() model.Host {
	return model.Host{ID: "h2", Label: "test-host-2", Address: "192.0.2.2", Port: 22}
}

func TestEnterStartsEmbeddedSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	if err := store.Save(&model.Store{Hosts: []model.Host{h}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if len(mm.sessions) != 1 {
		t.Fatal("expected one session started")
	}
	if !mm.inSessionMode {
		t.Fatal("expected inSessionMode set")
	}
	defer func() { _ = mm.sessions[0].sess.Close() }()
	if cmd == nil {
		t.Fatal("expected a non-nil cmd (waitSessionDone + redraw tick)")
	}
	if mm.homeView {
		t.Fatal("expected homeView left when starting a session")
	}
}

func TestStartEmbeddedSessionRecordsLastConnected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	if err := store.Save(&model.Store{Hosts: []model.Host{h}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Hosts[0].LastConnectedAt.IsZero() {
		t.Fatal("expected LastConnectedAt recorded before the session was even confirmed connected")
	}
}

func TestStartEmbeddedSessionFallsBackToFullScreenWhenSSHUnavailable(t *testing.T) {
	// Point PATH at an empty directory so exec.LookPath("ssh") fails
	// deterministically inside connect.Command/sshsession.Start, without
	// touching the network or depending on any real ssh binary at all.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	if err := store.Save(&model.Store{Hosts: []model.Host{h}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, cmd := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	if len(mm.sessions) != 0 {
		t.Fatal("expected no session when ssh isn't available")
	}
	// The fallback (connectTo) also can't find ssh, so it bails out
	// early too — nil cmd, and crucially no LastConnectedAt recorded,
	// since neither path actually attempted a connection.
	if cmd != nil {
		t.Fatal("expected nil cmd: both the embedded path and its full-screen fallback fail to find ssh")
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Hosts[0].LastConnectedAt.IsZero() {
		t.Fatal("expected LastConnectedAt NOT recorded when no connection was ever attempted")
	}
}

func TestActiveSessionCapturesInputInsteadOfQuitting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	// Regression: while a session is active, "q" must be forwarded to the
	// remote session (it might be a legitimate remote command), not
	// interpreted as minissh's own quit key.
	updated, cmd := mm.Update(keyRune('q'))
	mm2 := updated.(appModel)
	if len(mm2.sessions) != 1 || !mm2.inSessionMode {
		t.Fatal("expected session to remain active after pressing q")
	}
	if cmd != nil {
		t.Fatal("expected no tea.Quit (or any other) cmd from a forwarded keystroke")
	}
}

func TestUpdateActiveSessionDoesNotPanicOnUnrecognizedKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	_, cmd := mm.updateActiveSession(tea.KeyMsg{Type: tea.KeyF20})
	if cmd != nil {
		t.Fatal("expected no cmd from an unmapped extended key")
	}
}

func TestSessionEndedMsgClearsActiveSessionAndReloadsStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	if err := store.Save(&model.Store{Hosts: []model.Host{h}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()
	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)

	// Simulate a host added elsewhere while "connected" — the reload
	// triggered by the session ending should pick it up, same as
	// connectFinishedMsg already does for the full-screen path.
	s, _ := store.Load()
	store.UpsertHost(s, model.Host{Label: "added-while-away", Address: "10.0.0.9"})
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	updated, _ = mm.Update(sessionEndedMsg{hostID: h.ID})
	mm2 := updated.(appModel)
	if len(mm2.sessions) != 0 {
		t.Fatal("expected session removed")
	}
	if mm2.inSessionMode {
		t.Fatal("expected inSessionMode cleared once no sessions remain")
	}
	if len(mm2.allHosts) != 2 {
		t.Fatalf("expected host list refreshed after session end, got %d hosts", len(mm2.allHosts))
	}
}

func TestSessionEndedMsgForBackgroundSessionLeavesCurrentUndisturbed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h1, h2 := testHostForSession(), testHostForSession2()
	if err := store.Save(&model.Store{Hosts: []model.Host{h1, h2}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel([]model.Host{h1, h2}, nil)
	m.applySizes()
	updated, _ := m.startEmbeddedSession(h1)
	mm := updated.(appModel)
	updated, _ = mm.startEmbeddedSession(h2)
	mm = updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	if mm.currentSessionIdx != 1 || mm.sessions[mm.currentSessionIdx].host.ID != h2.ID {
		t.Fatalf("expected h2's session focused after starting it, got idx=%d", mm.currentSessionIdx)
	}

	// h1's (background) session ends — h2 must remain focused.
	updated, _ = mm.Update(sessionEndedMsg{hostID: h1.ID})
	mm2 := updated.(appModel)
	if len(mm2.sessions) != 1 {
		t.Fatalf("expected 1 session remaining, got %d", len(mm2.sessions))
	}
	if mm2.sessions[mm2.currentSessionIdx].host.ID != h2.ID {
		t.Fatalf("expected h2 still focused, got %q", mm2.sessions[mm2.currentSessionIdx].host.ID)
	}
	if !mm2.inSessionMode {
		t.Fatal("expected inSessionMode to remain true — the focused session is still alive")
	}
}

func TestSessionRedrawMsgStopsAfterSessionEnds(t *testing.T) {
	m := newAppModel(nil, nil) // no active session
	_, cmd := m.Update(sessionRedrawMsg{})
	if cmd != nil {
		t.Fatal("expected sessionRedrawMsg to be dropped (no reschedule) once there's no active session")
	}
}

func TestSessionRedrawMsgReschedulesWhileActive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()
	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	_, cmd := mm.Update(sessionRedrawMsg{})
	if cmd == nil {
		t.Fatal("expected sessionRedrawMsg to reschedule another tick while a session is active")
	}
}

func TestMainViewRendersActiveSessionHeader(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()
	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	view := mm.mainView()
	if !strings.Contains(view, "SESSION") || !strings.Contains(view, h.Label) {
		t.Fatalf("expected view to show a SESSION header mentioning %q, got:\n%s", h.Label, view)
	}
}

func TestApplySizesResizesActiveSessionWithoutError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()
	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	mm.width, mm.height = 120, 40
	mm.applySizes() // must not panic with an active session present
}

func TestEnterOnAlreadyOpenHostReattachesInstead(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	if err := store.Save(&model.Store{Hosts: []model.Host{h}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()
	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	firstSess := mm.sessions[0].sess

	mm.inSessionMode = false // simulate having detached back to the host table
	updated, _ = mm.startEmbeddedSession(h)
	mm2 := updated.(appModel)
	defer func() { _ = mm2.sessions[0].sess.Close() }()

	if len(mm2.sessions) != 1 {
		t.Fatalf("expected re-pressing enter on an already-open host to reuse it, got %d sessions", len(mm2.sessions))
	}
	if mm2.sessions[0].sess != firstSess {
		t.Fatal("expected the same underlying session, not a new ssh process")
	}
	if !mm2.inSessionMode {
		t.Fatal("expected re-attaching to set inSessionMode")
	}
}

func TestSecondSessionKeepsFirstAlive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h1, h2 := testHostForSession(), testHostForSession2()
	m := newAppModel([]model.Host{h1, h2}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h1)
	mm := updated.(appModel)
	updated, _ = mm.startEmbeddedSession(h2)
	mm2 := updated.(appModel)
	defer func() {
		for _, ls := range mm2.sessions {
			_ = ls.sess.Close()
		}
	}()

	if len(mm2.sessions) != 2 {
		t.Fatalf("expected both sessions alive, got %d", len(mm2.sessions))
	}
	if mm2.currentSessionIdx != 1 {
		t.Fatalf("expected the newly-started session focused, got idx %d", mm2.currentSessionIdx)
	}
}

func TestCtrlRightLeftCyclesSessions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h1, h2 := testHostForSession(), testHostForSession2()
	m := newAppModel([]model.Host{h1, h2}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h1)
	mm := updated.(appModel)
	updated, _ = mm.startEmbeddedSession(h2)
	mm = updated.(appModel)
	defer func() {
		for _, ls := range mm.sessions {
			_ = ls.sess.Close()
		}
	}()

	if mm.currentSessionIdx != 1 {
		t.Fatalf("expected h2 focused after starting it, got idx %d", mm.currentSessionIdx)
	}

	updated, _ = mm.updateActiveSession(tea.KeyMsg{Type: tea.KeyCtrlRight})
	mm = updated.(appModel)
	if mm.currentSessionIdx != 0 {
		t.Fatalf("expected ctrl+right to wrap to idx 0, got %d", mm.currentSessionIdx)
	}

	updated, _ = mm.updateActiveSession(tea.KeyMsg{Type: tea.KeyCtrlLeft})
	mm = updated.(appModel)
	if mm.currentSessionIdx != 1 {
		t.Fatalf("expected ctrl+left to wrap back to idx 1, got %d", mm.currentSessionIdx)
	}
}

// TestCtrlRightLeftCyclesSessionsWithAltFlaggedVariant is a regression test:
// some terminals encode ctrl+arrow as ESC[1;7C/D (xterm's ctrl+alt modifier
// code) or urxvt's ESC[Oc/Od rather than the plain ESC[1;5C/D bubbletea
// otherwise expects. bubbletea still parses these as
// KeyCtrlRight/KeyCtrlLeft, but with Alt set — matching on km.String()
// (which then reads "alt+ctrl+right") missed this and silently forwarded
// the escape to the remote session instead of switching tabs.
func TestCtrlRightLeftCyclesSessionsWithAltFlaggedVariant(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h1, h2 := testHostForSession(), testHostForSession2()
	m := newAppModel([]model.Host{h1, h2}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h1)
	mm := updated.(appModel)
	updated, _ = mm.startEmbeddedSession(h2)
	mm = updated.(appModel)
	defer func() {
		for _, ls := range mm.sessions {
			_ = ls.sess.Close()
		}
	}()

	updated, _ = mm.updateActiveSession(tea.KeyMsg{Type: tea.KeyCtrlRight, Alt: true})
	mm = updated.(appModel)
	if mm.currentSessionIdx != 0 {
		t.Fatalf("expected alt-flagged ctrl+right to still cycle tabs, got idx %d", mm.currentSessionIdx)
	}
}

func TestCtrlBackslashDetachesWithoutClosing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := testHostForSession()
	m := newAppModel([]model.Host{h}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h)
	mm := updated.(appModel)
	defer func() { _ = mm.sessions[0].sess.Close() }()

	updated, _ = mm.updateActiveSession(tea.KeyMsg{Type: tea.KeyCtrlBackslash})
	mm2 := updated.(appModel)
	if mm2.inSessionMode {
		t.Fatal("expected ctrl+\\ to detach (inSessionMode false)")
	}
	if len(mm2.sessions) != 1 {
		t.Fatal("expected the session to keep running in the background after detaching")
	}
	select {
	case <-mm2.sessions[0].sess.Done():
		t.Fatal("expected the session to still be alive after detaching, not closed")
	default:
	}
}

func TestCloseSessionForHostClosesTheRightOne(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h1, h2 := testHostForSession(), testHostForSession2()
	m := newAppModel([]model.Host{h1, h2}, nil)
	m.applySizes()

	updated, _ := m.startEmbeddedSession(h1)
	mm := updated.(appModel)
	updated, _ = mm.startEmbeddedSession(h2)
	mm = updated.(appModel)
	defer func() {
		for _, ls := range mm.sessions {
			_ = ls.sess.Close()
		}
	}()

	// closeSessionForHost only kills the process; it doesn't touch
	// m.sessions itself (see its doc comment) — simulate the async
	// sessionEndedMsg that would normally follow, rather than blocking the
	// test on the real readLoop noticing.
	mm.closeSessionForHost(h1.ID)

	updated, _ = mm.Update(sessionEndedMsg{hostID: h1.ID})
	mm2 := updated.(appModel)
	if len(mm2.sessions) != 1 || mm2.sessions[0].host.ID != h2.ID {
		t.Fatalf("expected only h2's session remaining, got %+v", mm2.sessions)
	}
}
