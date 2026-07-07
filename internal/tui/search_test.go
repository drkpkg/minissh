package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danieluremix/minissh/internal/model"
)

func testSearchHosts() []model.Host {
	return []model.Host{
		{Label: "web-prod-01", Address: "10.0.1.10", Username: "root", Tags: []string{"prod", "web"}},
		{Label: "web-prod-02", Address: "10.0.1.11", Username: "root", Tags: []string{"prod", "web"}},
		{Label: "db-staging", Address: "10.0.2.20", Username: "admin", Tags: []string{"staging", "db"}},
	}
}

func TestSearchHostsEmptyQueryReturnsUnchanged(t *testing.T) {
	hosts := testSearchHosts()
	got := searchHosts(hosts, nil, "", searchAll)
	if len(got) != len(hosts) {
		t.Fatalf("expected unchanged host list, got %d hosts", len(got))
	}
}

func TestSearchHostsMatchesByName(t *testing.T) {
	got := searchHosts(testSearchHosts(), nil, "webprod", searchName)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(got), got)
	}
	for _, h := range got {
		if h.Label != "web-prod-01" && h.Label != "web-prod-02" {
			t.Fatalf("unexpected match: %+v", h)
		}
	}
}

func TestSearchHostsMatchesByAddress(t *testing.T) {
	got := searchHosts(testSearchHosts(), nil, "10.0.2.20", searchAddress)
	if len(got) != 1 || got[0].Label != "db-staging" {
		t.Fatalf("expected db-staging match, got %+v", got)
	}
}

func TestSearchHostsMatchesByTag(t *testing.T) {
	got := searchHosts(testSearchHosts(), nil, "staging", searchTags)
	if len(got) != 1 || got[0].Label != "db-staging" {
		t.Fatalf("expected db-staging match by tag, got %+v", got)
	}
}

func TestSearchHostsMatchesByGroup(t *testing.T) {
	groups := []model.Group{{ID: "g1", Name: "prod"}}
	hosts := []model.Host{
		{Label: "a", GroupID: "g1"},
		{Label: "b"},
	}
	got := searchHosts(hosts, groups, "prod", searchGroup)
	if len(got) != 1 || got[0].Label != "a" {
		t.Fatalf("expected host 'a' matched by group, got %+v", got)
	}
}

func TestSearchHostsAllScansEveryField(t *testing.T) {
	hosts := testSearchHosts()
	// "admin" only appears as a username, not name/address/tag — searchAll
	// should still find it.
	got := searchHosts(hosts, nil, "admin", searchAll)
	if len(got) != 1 || got[0].Label != "db-staging" {
		t.Fatalf("expected db-staging matched via username under searchAll, got %+v", got)
	}
}

func TestSearchHostsNoMatchReturnsEmpty(t *testing.T) {
	got := searchHosts(testSearchHosts(), nil, "zzz-nonexistent-zzz", searchAll)
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %d", len(got))
	}
}

// --- app-level wiring -------------------------------------------------------

func TestSlashEntersSearchMode(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	updated, cmd := m.Update(keyRune('/'))
	mm := updated.(appModel)
	if !mm.searching {
		t.Fatal("expected searching=true after /")
	}
	if cmd == nil {
		t.Fatal("expected textinput.Blink cmd")
	}
}

func TestSearchFiltersHostTableLive(t *testing.T) {
	hosts, groups := testHostsAndGroups() // web-1 (g-web) and other (ungrouped)
	m := newAppModel(hosts, groups)
	m.applySizes()

	updated, _ := m.Update(keyRune('/'))
	mm := updated.(appModel)

	updated, _ = mm.Update(keyRune('w'))
	mm = updated.(appModel)
	updated, _ = mm.Update(keyRune('e'))
	mm = updated.(appModel)
	updated, _ = mm.Update(keyRune('b'))
	mm = updated.(appModel)

	if len(mm.hosts.hosts) != 1 || mm.hosts.hosts[0].Label != "web-1" {
		t.Fatalf("expected only web-1 matched, got %+v", mm.hosts.hosts)
	}
}

func TestSearchEscClearsQueryAndRestoresFullList(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	updated, _ := m.Update(keyRune('/'))
	mm := updated.(appModel)
	updated, _ = mm.Update(keyRune('w'))
	mm = updated.(appModel)

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(appModel)
	if mm.searching {
		t.Fatal("expected searching=false after esc")
	}
	if mm.searchQuery != "" {
		t.Fatalf("expected query cleared, got %q", mm.searchQuery)
	}
	if len(mm.hosts.hosts) != len(hosts) {
		t.Fatalf("expected full host list restored, got %d", len(mm.hosts.hosts))
	}
}

func TestSearchEnterCommitsAndFocusesHosts(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()
	m.focus = focusGroups

	updated, _ := m.Update(keyRune('/'))
	mm := updated.(appModel)
	updated, _ = mm.Update(keyRune('w'))
	mm = updated.(appModel)

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(appModel)
	if mm.searching {
		t.Fatal("expected searching=false after enter")
	}
	if mm.focus != focusHosts {
		t.Fatalf("expected focus moved to focusHosts, got %v", mm.focus)
	}
	// Query (and thus the filter) should still be applied after commit.
	if mm.searchQuery != "w" {
		t.Fatalf("expected query preserved, got %q", mm.searchQuery)
	}
}

func TestSearchTabCyclesField(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)

	updated, _ := m.Update(keyRune('/'))
	mm := updated.(appModel)
	if mm.searchField != searchAll {
		t.Fatalf("expected default searchAll, got %v", mm.searchField)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(appModel)
	if mm.searchField != searchName {
		t.Fatalf("expected searchName after tab, got %v", mm.searchField)
	}
}
