package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/danieluremix/minissh/internal/importer"
	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/sources"
	"github.com/danieluremix/minissh/internal/store"
)

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// --- focus / panel navigation -------------------------------------------

func TestLAndTabCycleFocusForward(t *testing.T) {
	m := newAppModel(nil, nil)
	if m.focus != focusHosts {
		t.Fatalf("expected default focus focusHosts, got %v", m.focus)
	}

	updated, _ := m.Update(keyRune('l'))
	mm := updated.(appModel)
	if mm.focus != focusDetails {
		t.Fatalf("expected focusDetails after l, got %v", mm.focus)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(appModel)
	if mm.focus != focusGroups {
		t.Fatalf("expected focusGroups after tab (wraps around), got %v", mm.focus)
	}
}

func TestHCyclesFocusBackward(t *testing.T) {
	m := newAppModel(nil, nil)
	updated, _ := m.Update(keyRune('h'))
	mm := updated.(appModel)
	if mm.focus != focusGroups {
		t.Fatalf("expected focusGroups after h from focusHosts, got %v", mm.focus)
	}
}

func TestQuitFromMainView(t *testing.T) {
	m := newAppModel(nil, nil)
	_, cmd := m.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("expected a cmd (tea.Quit) from q")
	}
}

// --- groups panel ---------------------------------------------------------

func testHostsAndGroups() ([]model.Host, []model.Group) {
	groups := []model.Group{
		{ID: "g-prod", Name: "prod"},
		{ID: "g-web", Name: "web", ParentID: "g-prod"},
	}
	hosts := []model.Host{
		{ID: "h1", Label: "web-1", Address: "10.0.0.1", GroupID: "g-web"},
		{ID: "h2", Label: "other", Address: "10.0.0.2"},
	}
	return hosts, groups
}

func TestGroupsFocusFiltersHostTable(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.focus = focusGroups
	m.applySizes() // establish non-zero table size for View/selection to behave

	// cursor starts on "All Hosts" (root) -> both hosts visible.
	if len(m.hosts.hosts) != 2 {
		t.Fatalf("expected 2 hosts before filtering, got %d", len(m.hosts.hosts))
	}

	// Move down twice: root -> prod -> web.
	updated, _ := m.Update(keyRune('j'))
	mm := updated.(appModel)
	updated, _ = mm.Update(keyRune('j'))
	mm = updated.(appModel)

	g, ok := mm.groups.Selected()
	if !ok || g.Name != "web" {
		t.Fatalf("expected 'web' group selected, got %+v ok=%v", g, ok)
	}
	if len(mm.hosts.hosts) != 1 || mm.hosts.hosts[0].Label != "web-1" {
		t.Fatalf("expected host table filtered to web-1, got %+v", mm.hosts.hosts)
	}
}

func TestGroupsEnterDrillsIntoHosts(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.focus = focusGroups

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if mm.focus != focusHosts {
		t.Fatalf("expected enter to drill focus into focusHosts, got %v", mm.focus)
	}
}

func TestGroupsSpaceTogglesExpand(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.focus = focusGroups

	before := len(m.groups.nodes)
	updated, _ := m.Update(keyRune('j')) // move onto "prod" (has children)
	mm := updated.(appModel)
	updated, _ = mm.Update(keyRune(' ')) // collapse it
	mm = updated.(appModel)

	if len(mm.groups.nodes) >= before {
		t.Fatalf("expected fewer visible nodes after collapsing prod, before=%d after=%d", before, len(mm.groups.nodes))
	}
}

// --- hosts panel ------------------------------------------------------------

func TestHostsEnterConnectsWithoutQuittingTheApp(t *testing.T) {
	// Regression: minissh used to quit on connect (tea.Quit) and let the
	// CLI exec-replace itself with ssh. It now suspends via
	// tea.ExecProcess and stays running so it's still there once the ssh
	// session ends — enter must NOT return tea.Quit.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	if err := store.Save(&model.Store{Hosts: hosts, Groups: groups}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	m.applySizes()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd (tea.ExecProcess) from pressing enter on a selected host")
	}
}

func TestConnectToRecordsLastConnectedBeforeLaunching(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	if err := store.Save(&model.Store{Hosts: hosts, Groups: groups}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	target := hosts[0]

	_, cmd := m.connectTo(target)
	if cmd == nil {
		t.Fatal("expected a non-nil cmd from connectTo")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, h := range loaded.Hosts {
		if h.ID == target.ID && h.LastConnectedAt.IsZero() {
			t.Fatal("expected LastConnectedAt recorded before the ssh command is launched")
		}
	}
}

func TestConnectFinishedMsgReloadsFromStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	if err := store.Save(&model.Store{Hosts: hosts, Groups: groups}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	m.applySizes()

	// Simulate a host being added by another process while "connected"
	// (e.g. via `minissh add` in another terminal) — refresh on return
	// should pick it up.
	s, _ := store.Load()
	store.UpsertHost(s, model.Host{Label: "added-while-away", Address: "10.0.0.9"})
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	updated, _ := m.Update(connectFinishedMsg{})
	mm := updated.(appModel)
	if len(mm.allHosts) != len(hosts)+1 {
		t.Fatalf("expected host list refreshed after connectFinishedMsg, got %d hosts", len(mm.allHosts))
	}
}

func TestHostsSpaceTogglesFavorite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, groups := testHostsAndGroups()
	s := &model.Store{Hosts: hosts, Groups: groups}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := newAppModel(hosts, groups)
	m.applySizes()

	updated, _ := m.Update(keyRune(' '))
	mm := updated.(appModel)

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	found := false
	for _, h := range loaded.Hosts {
		if h.Favorite {
			found = true
		}
	}
	if !found {
		t.Fatal("expected some host marked favorite in the store")
	}

	sel, ok := mm.hosts.Selected()
	if !ok {
		t.Fatal("expected a selected host in the table")
	}
	if !sel.Favorite {
		t.Fatal("expected the in-memory table row to reflect the favorite toggle immediately")
	}
}

func TestHostsSCyclesSort(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	if m.hosts.sortCol != sortByName {
		t.Fatalf("expected default sort by name, got %v", m.hosts.sortCol)
	}
	updated, _ := m.Update(keyRune('s'))
	mm := updated.(appModel)
	if mm.hosts.sortCol != sortByHost {
		t.Fatalf("expected sort cycled to sortByHost, got %v", mm.hosts.sortCol)
	}
}

// --- import wizard (overlay) ------------------------------------------------

func TestPressIEntersSourcePicker(t *testing.T) {
	m := newAppModel(nil, nil)
	updated, _ := m.Update(keyRune('i'))
	mm := updated.(appModel)
	if mm.overlay != overlaySourcePicker {
		t.Fatalf("expected overlaySourcePicker, got %v", mm.overlay)
	}
}

func TestSourcePickerQReturnsToMainInsteadOfQuitting(t *testing.T) {
	// Regression test: bubbles/list's own default KeyMap.Quit is bound to
	// both "q" and "esc". Before DisableQuitKeybindings + an explicit "q"
	// case in updateSourcePicker, an unhandled "q" here fell through to
	// the embedded list's Update and quit the whole program instead of
	// backing out of the picker.
	m := newAppModel(nil, nil)
	m.overlay = overlaySourcePicker
	updated, cmd := m.Update(keyRune('q'))
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone, got %v", mm.overlay)
	}
	if cmd != nil {
		t.Fatal("expected no cmd (in particular, no tea.Quit) from pressing q in the source picker")
	}
}

func TestSourcePickerEscReturnsToMain(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlaySourcePicker
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone, got %v", mm.overlay)
	}
}

func TestSourcePickerEnterOnFileSourceEntersFilePrompt(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlaySourcePicker
	// sources.All's first entry (csv) is selected by default.
	if !m.sourceList.SelectedItem().(sourceItem).source.RequiresFile {
		t.Fatal("test assumes the default-selected source requires a file")
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if mm.overlay != overlayFilePrompt {
		t.Fatalf("expected overlayFilePrompt, got %v", mm.overlay)
	}
	if mm.chosenSource.ID != "csv" {
		t.Fatalf("expected csv source chosen, got %q", mm.chosenSource.ID)
	}
	if cmd == nil {
		t.Fatal("expected a textinput.Blink cmd")
	}
}

func TestFilePromptEnterRunsImportAndReachesConfirm(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlayFilePrompt
	m.chosenSource, _ = lookupSource(t, "csv")
	m.filePrompt = textinput.New()
	m.filePrompt.SetValue("../importer/testdata/termius_export.csv")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(appModel)
	if mm.overlay != overlayConfirm {
		t.Fatalf("expected overlayConfirm, got %v", mm.overlay)
	}
	if mm.importErr != nil {
		t.Fatalf("unexpected import error: %v", mm.importErr)
	}
	if mm.importResult == nil || len(mm.importResult.Hosts) != 3 {
		t.Fatalf("expected 3 hosts from fixture, got %+v", mm.importResult)
	}
}

func TestFilePromptEscReturnsToSourcePicker(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlayFilePrompt
	m.filePrompt = textinput.New()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(appModel)
	if mm.overlay != overlaySourcePicker {
		t.Fatalf("expected overlaySourcePicker, got %v", mm.overlay)
	}
}

func TestConfirmYPersistsAndReturnsToMainWithRefreshedHosts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newAppModel(nil, nil)
	m.overlay = overlayConfirm
	m.chosenSource, _ = lookupSource(t, "csv") // RequiresFile true -> StoreSecrets false
	m.importResult = &importer.Result{
		Hosts: []model.Host{{Label: "web-1", Address: "10.0.0.1", Port: 22}},
	}

	updated, _ := m.Update(keyRune('y'))
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone, got %v", mm.overlay)
	}
	if mm.importErr != nil {
		t.Fatalf("unexpected error: %v", mm.importErr)
	}

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if len(s.Hosts) != 1 || s.Hosts[0].Label != "web-1" {
		t.Fatalf("expected host persisted, got %+v", s.Hosts)
	}
	if len(mm.allHosts) != 1 {
		t.Fatalf("expected in-memory host list refreshed, got %d", len(mm.allHosts))
	}
	if len(mm.hosts.hosts) != 1 {
		t.Fatalf("expected host table refreshed, got %d", len(mm.hosts.hosts))
	}
}

func TestConfirmOtherKeyCancelsWithoutPersisting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newAppModel(nil, nil)
	m.overlay = overlayConfirm
	m.importResult = &importer.Result{
		Hosts: []model.Host{{Label: "web-1", Address: "10.0.0.1"}},
	}

	updated, _ := m.Update(keyRune('n'))
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone, got %v", mm.overlay)
	}
	if mm.importResult != nil {
		t.Fatal("expected importResult cleared on cancel")
	}

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if len(s.Hosts) != 0 {
		t.Fatalf("expected nothing persisted, got %+v", s.Hosts)
	}
}

func TestConfirmErrorStateAnyKeyReturnsToMain(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlayConfirm
	m.importErr = errors.New("boom")

	updated, _ := m.Update(keyRune('x'))
	mm := updated.(appModel)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlayNone, got %v", mm.overlay)
	}
	if mm.importErr != nil {
		t.Fatal("expected importErr cleared")
	}
}

func TestConfirmViewRendersHostsAndSkipped(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlayConfirm
	m.chosenSource, _ = lookupSource(t, "csv")
	m.importResult = &importer.Result{
		Hosts:   []model.Host{{Label: "web-1", Address: "10.0.0.1", Port: 22}},
		Skipped: []string{"row 4: bad protocol"},
	}

	view := m.View()
	if !strings.Contains(view, "web-1") {
		t.Fatalf("expected view to mention web-1, got: %s", view)
	}
	if !strings.Contains(view, "1 entries skipped") {
		t.Fatalf("expected view to mention skipped count, got: %s", view)
	}
}

func TestConfirmViewRendersError(t *testing.T) {
	m := newAppModel(nil, nil)
	m.overlay = overlayConfirm
	m.importErr = errors.New("vault not found")

	view := m.View()
	if !strings.Contains(view, "vault not found") {
		t.Fatalf("expected view to mention the error, got: %s", view)
	}
}

func lookupSource(t *testing.T, id string) (sources.Source, bool) {
	t.Helper()
	for _, s := range buildSourceItems() {
		si := s.(sourceItem)
		if si.source.ID == id {
			return si.source, true
		}
	}
	t.Fatalf("source %q not found", id)
	return sources.Source{}, false
}
