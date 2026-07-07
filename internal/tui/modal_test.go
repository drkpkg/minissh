package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/store"
)

// --- hostForm unit tests -----------------------------------------------------

func TestHostFormToHostRequiresAddress(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	if _, _, err := f.toHost(); err == nil {
		t.Fatal("expected error for missing address")
	}
}

func TestHostFormToHostDefaultsPortTo22(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	h, _, err := f.toHost()
	if err != nil {
		t.Fatalf("toHost: %v", err)
	}
	if h.Port != 22 {
		t.Fatalf("expected default port 22, got %d", h.Port)
	}
}

func TestHostFormToHostRejectsInvalidPort(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	f.inputs[fieldPort].SetValue("not-a-number")
	if _, _, err := f.toHost(); err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestHostFormToHostLabelFallsBackToAddress(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	h, _, err := f.toHost()
	if err != nil {
		t.Fatalf("toHost: %v", err)
	}
	if h.Label != "10.0.0.1" {
		t.Fatalf("expected label to fall back to address, got %q", h.Label)
	}
}

func TestHostFormToHostKeyModeSetsKeyIdentity(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	f.authMode = authKey
	f.inputs[fieldSecret].SetValue("~/.ssh/id_ed25519")
	h, password, err := f.toHost()
	if err != nil {
		t.Fatalf("toHost: %v", err)
	}
	if h.Identity.Kind != model.IdentityKey || h.Identity.KeyPath != "~/.ssh/id_ed25519" {
		t.Fatalf("expected key identity, got %+v", h.Identity)
	}
	if password != "" {
		t.Fatalf("expected no password for key auth, got %q", password)
	}
}

func TestHostFormToHostKeyModeRequiresKeyPath(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	f.authMode = authKey
	if _, _, err := f.toHost(); err == nil {
		t.Fatal("expected error for missing key path")
	}
}

func TestHostFormToHostPasswordModeReturnsPassword(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	f.authMode = authPassword
	f.inputs[fieldSecret].SetValue("hunter2")
	h, password, err := f.toHost()
	if err != nil {
		t.Fatalf("toHost: %v", err)
	}
	if h.Identity.Kind != model.IdentityPassword {
		t.Fatalf("expected password identity, got %+v", h.Identity)
	}
	if password != "hunter2" {
		t.Fatalf("expected password 'hunter2', got %q", password)
	}
}

func TestHostFormToHostPasswordModeRequiresPasswordOnAdd(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.inputs[fieldAddress].SetValue("10.0.0.1")
	f.authMode = authPassword
	if _, _, err := f.toHost(); err == nil {
		t.Fatal("expected error for missing password on a new host")
	}
}

func TestHostFormToHostPasswordModeBlankOnEditKeepsExisting(t *testing.T) {
	// Editing a host that already uses password auth: leaving the
	// (always-blank) password field empty must not error, and must signal
	// "keep existing" via an empty returned password rather than failing
	// validation or wiping the stored secret.
	existing := model.Host{ID: "h1", Address: "10.0.0.1", Identity: model.Identity{Kind: model.IdentityPassword}}
	f := newHostForm(true, existing, "")
	if f.authMode != authPassword || !f.originalAuthPassword {
		t.Fatalf("expected form primed for password auth, got authMode=%v originalAuthPassword=%v", f.authMode, f.originalAuthPassword)
	}
	if f.inputs[fieldSecret].Value() != "" {
		t.Fatal("expected the password field to never be prefilled with the real secret")
	}

	h, password, err := f.toHost()
	if err != nil {
		t.Fatalf("toHost: %v", err)
	}
	if h.Identity.Kind != model.IdentityPassword {
		t.Fatalf("expected identity to remain password, got %+v", h.Identity)
	}
	if password != "" {
		t.Fatalf("expected empty password (meaning 'keep existing'), got %q", password)
	}
}

func TestHostFormAuthModeCyclesAndWraps(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	if f.authMode != authAgent {
		t.Fatalf("expected default authAgent, got %v", f.authMode)
	}
	f.CycleAuthMode(1)
	if f.authMode != authKey {
		t.Fatalf("expected authKey, got %v", f.authMode)
	}
	f.CycleAuthMode(1)
	if f.authMode != authPassword {
		t.Fatalf("expected authPassword, got %v", f.authMode)
	}
	f.CycleAuthMode(1)
	if f.authMode != authAgent {
		t.Fatalf("expected wrap back to authAgent, got %v", f.authMode)
	}
	f.CycleAuthMode(-1)
	if f.authMode != authPassword {
		t.Fatalf("expected wrap backward to authPassword, got %v", f.authMode)
	}
}

func TestHostFormNextSkipsSecretFieldWhenAgent(t *testing.T) {
	f := newHostForm(false, model.Host{}, "") // authAgent by default
	f.focusIdx = int(fieldAuthMode)
	f.Next()
	if f.focusIdx != int(fieldLabel) {
		t.Fatalf("expected Next() to skip fieldSecret and wrap to fieldLabel, got %d", f.focusIdx)
	}
}

func TestHostFormNextVisitsSecretFieldWhenKey(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	f.authMode = authKey
	f.focusIdx = int(fieldAuthMode)
	f.Next()
	if f.focusIdx != int(fieldSecret) {
		t.Fatalf("expected Next() to land on fieldSecret, got %d", f.focusIdx)
	}
}

func TestNewHostFormPrefillsAuthModeFromExistingIdentity(t *testing.T) {
	h := model.Host{Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "/x"}}
	f := newHostForm(true, h, "")
	if f.authMode != authKey {
		t.Fatalf("expected authKey, got %v", f.authMode)
	}
	if f.inputs[fieldSecret].Value() != "/x" {
		t.Fatalf("expected key path prefilled, got %q", f.inputs[fieldSecret].Value())
	}
}

func TestHostFormPrefillsFromExistingHost(t *testing.T) {
	h := model.Host{ID: "h1", Label: "web-1", Address: "10.0.0.1", Port: 2222, Username: "root"}
	f := newHostForm(true, h, "prod")
	if f.inputs[fieldLabel].Value() != "web-1" {
		t.Fatalf("expected prefilled label, got %q", f.inputs[fieldLabel].Value())
	}
	if f.inputs[fieldPort].Value() != "2222" {
		t.Fatalf("expected prefilled port, got %q", f.inputs[fieldPort].Value())
	}
	if f.hostID != "h1" {
		t.Fatalf("expected hostID carried over, got %q", f.hostID)
	}
}

func TestHostFormNextPrevCyclesFocus(t *testing.T) {
	f := newHostForm(false, model.Host{}, "")
	if f.focusIdx != int(fieldLabel) {
		t.Fatalf("expected initial focus on label, got %d", f.focusIdx)
	}
	f.Next()
	if f.focusIdx != int(fieldAddress) {
		t.Fatalf("expected focus on address after Next, got %d", f.focusIdx)
	}
	f.Prev()
	if f.focusIdx != int(fieldLabel) {
		t.Fatalf("expected focus back on label after Prev, got %d", f.focusIdx)
	}
	f.Prev() // wraps to the last field reachable in agent mode (fieldSecret is skipped)
	if f.focusIdx != int(fieldAuthMode) {
		t.Fatalf("expected focus wrapped to fieldAuthMode, got %d", f.focusIdx)
	}
}

// --- app-level Add ------------------------------------------------------------

func TestAKeyOpensAddModal(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	updated, _ := m.Update(keyRune('a'))
	mm := updated.(appModel)
	if mm.overlay != overlayAddEdit {
		t.Fatalf("expected overlayAddEdit, got %v", mm.overlay)
	}
	if mm.hostForm.editing {
		t.Fatal("expected a new (non-editing) form")
	}
}

func TestAddModalEnterSavesNewHost(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	s := &model.Store{Hosts: hosts, Groups: groups}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	m.applySizes()
	m.overlay = overlayAddEdit
	m.hostForm = newHostForm(false, model.Host{}, "")
	m.hostForm.inputs[fieldLabel].SetValue("new-host")
	m.hostForm.inputs[fieldAddress].SetValue("10.0.9.9")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone after save, got %v", mm.overlay)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	found := false
	for _, h := range loaded.Hosts {
		if h.Label == "new-host" && h.Address == "10.0.9.9" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected new-host persisted, got %+v", loaded.Hosts)
	}
	if len(mm.allHosts) != len(hosts)+1 {
		t.Fatalf("expected in-memory host list refreshed with new host, got %d", len(mm.allHosts))
	}
}

func TestAddModalEnterWithoutAddressKeepsFormOpenWithError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newAppModel(nil, nil)
	m.overlay = overlayAddEdit
	m.hostForm = newHostForm(false, model.Host{}, "")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if mm.overlay != overlayAddEdit {
		t.Fatalf("expected form to stay open on validation error, got %v", mm.overlay)
	}
	if mm.hostForm.err == "" {
		t.Fatal("expected a validation error message set on the form")
	}
}

func TestAddModalEscCancels(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlayAddEdit
	m.hostForm = newHostForm(false, model.Host{}, "")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone after esc, got %v", mm.overlay)
	}
}

// --- app-level Edit -------------------------------------------------------

func TestEKeyOpensEditModalPrefilled(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	updated, _ := m.Update(keyRune('e'))
	mm := updated.(appModel)
	if mm.overlay != overlayAddEdit {
		t.Fatalf("expected overlayAddEdit, got %v", mm.overlay)
	}
	if !mm.hostForm.editing {
		t.Fatal("expected editing=true")
	}
	sel, _ := m.hosts.Selected()
	if mm.hostForm.inputs[fieldAddress].Value() != sel.Address {
		t.Fatalf("expected form prefilled with selected host's address, got %q", mm.hostForm.inputs[fieldAddress].Value())
	}
}

func TestEditModalRenamePreservesIdentityAndDoesNotDuplicate(t *testing.T) {
	// Regression: editing must find-and-mutate by ID, not upsert by
	// label+address (which would silently create a duplicate when the
	// label/address themselves are what's being changed).
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := &model.Store{}
	id := store.UpsertHost(s, model.Host{Label: "old-name", Address: "10.0.0.1", Tags: []string{"prod"}})
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(s.Hosts, s.Groups)
	m.applySizes()
	m.overlay = overlayAddEdit
	m.hostForm = newHostForm(true, s.Hosts[0], "")
	m.hostForm.inputs[fieldLabel].SetValue("new-name")
	m.hostForm.inputs[fieldAddress].SetValue("10.0.0.2")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected save to succeed, got overlay=%v err=%q", mm.overlay, mm.hostForm.err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Hosts) != 1 {
		t.Fatalf("expected still exactly 1 host (no duplicate), got %d: %+v", len(loaded.Hosts), loaded.Hosts)
	}
	if loaded.Hosts[0].ID != id {
		t.Fatalf("expected same host ID preserved, got %q want %q", loaded.Hosts[0].ID, id)
	}
	if loaded.Hosts[0].Label != "new-name" || loaded.Hosts[0].Address != "10.0.0.2" {
		t.Fatalf("expected rename to take effect, got %+v", loaded.Hosts[0])
	}
	if len(loaded.Hosts[0].Tags) != 1 || loaded.Hosts[0].Tags[0] != "prod" {
		t.Fatalf("expected tags (not exposed by the form) preserved, got %+v", loaded.Hosts[0].Tags)
	}
}

// --- app-level Delete -----------------------------------------------------

func TestDKeyOpensDeleteConfirm(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	updated, _ := m.Update(keyRune('d'))
	mm := updated.(appModel)
	if mm.overlay != overlayDeleteConfirm {
		t.Fatalf("expected overlayDeleteConfirm, got %v", mm.overlay)
	}
	if mm.deleteTarget == nil {
		t.Fatal("expected deleteTarget set")
	}
}

func TestDeleteConfirmYRemovesHost(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	s := &model.Store{Hosts: hosts, Groups: groups}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	m.applySizes()
	target := hosts[0]
	m.deleteTarget = &target
	m.overlay = overlayDeleteConfirm

	updated, _ := m.Update(keyRune('y'))
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone after delete, got %v", mm.overlay)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Hosts) != len(hosts)-1 {
		t.Fatalf("expected 1 host removed, got %d remaining", len(loaded.Hosts))
	}
	for _, h := range loaded.Hosts {
		if h.ID == target.ID {
			t.Fatal("expected deleted host to be gone")
		}
	}
}

func TestDeleteConfirmOtherKeyCancels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	s := &model.Store{Hosts: hosts, Groups: groups}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	target := hosts[0]
	m.deleteTarget = &target
	m.overlay = overlayDeleteConfirm

	updated, _ := m.Update(keyRune('n'))
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone, got %v", mm.overlay)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Hosts) != len(hosts) {
		t.Fatalf("expected nothing deleted, got %d hosts", len(loaded.Hosts))
	}
}

func TestDeleteConfirmViewMentionsHost(t *testing.T) {
	view := deleteConfirmView(model.Host{Label: "web-1", Address: "10.0.0.1"})
	if !strings.Contains(view, "web-1") || !strings.Contains(view, "10.0.0.1") {
		t.Fatalf("expected view to mention host label/address, got: %s", view)
	}
}

// --- render smoke -----------------------------------------------------------

func TestRenderModalCentersWithinCanvas(t *testing.T) {
	out := renderModal(40, 10, "hi")
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
}
