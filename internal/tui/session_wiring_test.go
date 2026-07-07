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
	if mm.activeSession == nil {
		t.Fatal("expected activeSession to be set")
	}
	defer func() { _ = mm.activeSession.Close() }()
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
	defer func() { _ = mm.activeSession.Close() }()

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
	if mm.activeSession != nil {
		t.Fatal("expected no activeSession when ssh isn't available")
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
	defer func() { _ = mm.activeSession.Close() }()

	// Regression: while a session is active, "q" must be forwarded to the
	// remote session (it might be a legitimate remote command), not
	// interpreted as minissh's own quit key.
	updated, cmd := mm.Update(keyRune('q'))
	mm2 := updated.(appModel)
	if mm2.activeSession == nil {
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
	defer func() { _ = mm.activeSession.Close() }()

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

	// Simulate a host added elsewhere while "connected" — endSession's
	// reload should pick it up, same as connectFinishedMsg already does
	// for the full-screen path.
	s, _ := store.Load()
	store.UpsertHost(s, model.Host{Label: "added-while-away", Address: "10.0.0.9"})
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	updated, _ = mm.Update(sessionEndedMsg{})
	mm2 := updated.(appModel)
	if mm2.activeSession != nil {
		t.Fatal("expected activeSession cleared")
	}
	if len(mm2.allHosts) != 2 {
		t.Fatalf("expected host list refreshed after session end, got %d hosts", len(mm2.allHosts))
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
	defer func() { _ = mm.activeSession.Close() }()

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
	defer func() { _ = mm.activeSession.Close() }()

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
	defer func() { _ = mm.activeSession.Close() }()

	mm.width, mm.height = 120, 40
	mm.applySizes() // must not panic with an active session present
}
