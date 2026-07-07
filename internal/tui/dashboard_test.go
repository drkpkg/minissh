package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/drkpkg/minissh/internal/model"
)

func TestDashboardViewShowsFavoritesAndOffline(t *testing.T) {
	hosts := []model.Host{
		{ID: "1", Label: "fav-host", Address: "10.0.0.1", Favorite: true},
		{ID: "2", Label: "down-host", Address: "10.0.0.2"},
		{ID: "3", Label: "plain-host", Address: "10.0.0.3"},
	}
	statuses := map[string]bool{"2": false, "3": true}

	view := dashboardView(hosts, nil, statuses)
	if !strings.Contains(view, "fav-host") {
		t.Fatalf("expected favorites section to mention fav-host, got:\n%s", view)
	}
	if !strings.Contains(view, "down-host") {
		t.Fatalf("expected offline section to mention down-host, got:\n%s", view)
	}
}

func TestDashboardViewEmptySectionsShowNone(t *testing.T) {
	view := dashboardView(nil, nil, nil)
	if !strings.Contains(view, "none") {
		t.Fatalf("expected empty sections to say 'none', got:\n%s", view)
	}
}

func TestRecentlyConnectedOrdersMostRecentFirst(t *testing.T) {
	now := time.Now()
	hosts := []model.Host{
		{Label: "old", LastConnectedAt: now.Add(-time.Hour)},
		{Label: "never"}, // zero value, excluded
		{Label: "new", LastConnectedAt: now},
	}
	got := recentlyConnected(hosts, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 (excluding never-connected), got %d", len(got))
	}
	if got[0].Label != "new" || got[1].Label != "old" {
		t.Fatalf("expected new before old, got %+v", got)
	}
}

func TestRecentlyConnectedRespectsLimit(t *testing.T) {
	now := time.Now()
	var hosts []model.Host
	for i := 0; i < 10; i++ {
		hosts = append(hosts, model.Host{Label: "h", LastConnectedAt: now.Add(time.Duration(i) * time.Minute)})
	}
	got := recentlyConnected(hosts, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
}

func TestRecentlyAddedReturnsTailReversed(t *testing.T) {
	hosts := []model.Host{
		{Label: "first"}, {Label: "second"}, {Label: "third"},
	}
	got := recentlyAdded(hosts, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Label != "third" || got[1].Label != "second" {
		t.Fatalf("expected [third, second], got %+v", got)
	}
}

func TestRecentlyAddedHandlesFewerThanLimit(t *testing.T) {
	hosts := []model.Host{{Label: "only"}}
	got := recentlyAdded(hosts, 5)
	if len(got) != 1 || got[0].Label != "only" {
		t.Fatalf("expected [only], got %+v", got)
	}
}

func TestRelativeTime(t *testing.T) {
	if relativeTime(time.Time{}) != "never" {
		t.Fatal("expected 'never' for zero time")
	}
	if got := relativeTime(time.Now().Add(-30 * time.Second)); got != "just now" {
		t.Fatalf("expected 'just now', got %q", got)
	}
	if got := relativeTime(time.Now().Add(-5 * time.Minute)); got != "5m ago" {
		t.Fatalf("expected '5m ago', got %q", got)
	}
	if got := relativeTime(time.Now().Add(-3 * time.Hour)); got != "3h ago" {
		t.Fatalf("expected '3h ago', got %q", got)
	}
	if got := relativeTime(time.Now().Add(-48 * time.Hour)); got != "2d ago" {
		t.Fatalf("expected '2d ago', got %q", got)
	}
}

func TestEmptyStateViewMentionsAddAndImport(t *testing.T) {
	view := emptyStateView()
	if !strings.Contains(view, "No hosts yet") {
		t.Fatalf("expected onboarding heading, got:\n%s", view)
	}
	if !strings.Contains(view, "add a host") || !strings.Contains(view, "import") {
		t.Fatalf("expected add/import hints, got:\n%s", view)
	}
}

// --- app-level dashboard wiring ----------------------------------------------

func TestNewAppModelStartsInHomeViewWhenHostsExist(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	if !m.homeView {
		t.Fatal("expected homeView=true on launch with existing hosts")
	}
}

func TestNewAppModelSkipsHomeViewWhenNoHosts(t *testing.T) {
	m := newAppModel(nil, nil)
	if m.homeView {
		t.Fatal("expected homeView=false with zero hosts (empty state takes over instead)")
	}
}

func TestHostsPanelInteractionLeavesHomeView(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()
	if !m.homeView {
		t.Fatal("expected homeView=true initially")
	}

	updated, _ := m.Update(keyRune('j'))
	mm := updated.(appModel)
	if mm.homeView {
		t.Fatal("expected homeView=false after interacting with the hosts panel")
	}
}

func TestMainViewRendersDashboardWhenHomeViewActive(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	view := m.mainView()
	if !strings.Contains(view, "DASHBOARD") {
		t.Fatalf("expected DASHBOARD header in home view, got:\n%s", view)
	}
}

func TestMainViewRendersEmptyStateWithNoHosts(t *testing.T) {
	m := newAppModel(nil, nil)
	m.applySizes()
	view := m.mainView()
	if !strings.Contains(view, "No hosts yet") {
		t.Fatalf("expected empty state, got:\n%s", view)
	}
}
