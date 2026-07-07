package tui

import (
	"strings"
	"testing"

	"github.com/drkpkg/minissh/internal/model"
)

func TestHostsTableSortByNameDefault(t *testing.T) {
	ht := newHostsTable()
	ht.SetHosts([]model.Host{
		{ID: "1", Label: "zeta", Address: "10.0.0.1"},
		{ID: "2", Label: "alpha", Address: "10.0.0.2"},
	})
	if ht.hosts[0].Label != "alpha" || ht.hosts[1].Label != "zeta" {
		t.Fatalf("expected alpha before zeta, got %+v", ht.hosts)
	}
}

func TestHostsTableSortByHost(t *testing.T) {
	ht := newHostsTable()
	ht.SetHosts([]model.Host{
		{ID: "1", Label: "b", Address: "10.0.0.9"},
		{ID: "2", Label: "a", Address: "10.0.0.1"},
	})
	ht.sortCol = sortByHost
	ht.refresh()
	if ht.hosts[0].Address != "10.0.0.1" {
		t.Fatalf("expected sorted by address ascending, got %+v", ht.hosts)
	}
}

func TestHostsTableCycleSortWrapsAround(t *testing.T) {
	ht := newHostsTable()
	cols := []sortColumn{sortByName, sortByHost, sortByUser, sortByStatus}
	for _, want := range cols[1:] {
		ht.CycleSort()
		if ht.sortCol != want {
			t.Fatalf("expected sort col %v, got %v", want, ht.sortCol)
		}
	}
	ht.CycleSort() // wraps back to name
	if ht.sortCol != sortByName {
		t.Fatalf("expected sort to wrap back to sortByName, got %v", ht.sortCol)
	}
}

func TestHostsTableStatusSortGroupsOnlineBeforeUnknownBeforeOffline(t *testing.T) {
	ht := newHostsTable()
	ht.SetHosts([]model.Host{
		{ID: "offline", Label: "c"},
		{ID: "unknown", Label: "b"},
		{ID: "online", Label: "a"},
	})
	ht.SetStatuses(map[string]bool{"offline": false, "online": true})
	ht.sortCol = sortByStatus
	ht.refresh()

	got := []string{ht.hosts[0].ID, ht.hosts[1].ID, ht.hosts[2].ID}
	want := []string{"online", "unknown", "offline"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("status sort order = %v, want %v", got, want)
		}
	}
}

func TestHostsTableSetHostsResetsSelectionSafely(t *testing.T) {
	ht := newHostsTable()
	ht.SetHosts([]model.Host{{ID: "1", Label: "a"}, {ID: "2", Label: "b"}})
	ht.tbl.SetCursor(1)
	ht.SetHosts([]model.Host{{ID: "1", Label: "a"}}) // fewer rows now
	if _, ok := ht.Selected(); !ok {
		t.Fatal("expected Selected() to still return a valid host after shrinking the list")
	}
}

func TestStatusBadgeUnknownVsOnlineVsOffline(t *testing.T) {
	statuses := map[string]bool{"up": true, "down": false}
	if got := statusBadge(statuses, "missing"); !strings.Contains(got, "—") {
		t.Fatalf("expected unknown badge for unprobed host, got %q", got)
	}
	if got := statusBadge(statuses, "up"); !strings.Contains(got, "online") {
		t.Fatalf("expected online badge, got %q", got)
	}
	if got := statusBadge(statuses, "down"); !strings.Contains(got, "offline") {
		t.Fatalf("expected offline badge, got %q", got)
	}
}

func TestFavoriteBadge(t *testing.T) {
	if favoriteBadge(false) != "" {
		t.Fatal("expected empty badge for non-favorite")
	}
	if !strings.Contains(favoriteBadge(true), "★") {
		t.Fatal("expected star badge for favorite")
	}
}

func TestAuthTagAllKinds(t *testing.T) {
	cases := map[model.IdentityKind]bool{
		model.IdentityKey:      true,
		model.IdentityPassword: true,
		model.IdentityAgent:    true,
		"":                     false,
	}
	for kind, wantNonEmpty := range cases {
		got := authTag(kind) != ""
		if got != wantNonEmpty {
			t.Fatalf("authTag(%q) non-empty = %v, want %v", kind, got, wantNonEmpty)
		}
	}
}

func TestHostAddressColumnOmitsDefaultPort(t *testing.T) {
	if got := hostAddressColumn(model.Host{Address: "10.0.0.1", Port: 22}); got != "10.0.0.1" {
		t.Fatalf("hostAddressColumn = %q", got)
	}
	if got := hostAddressColumn(model.Host{Address: "10.0.0.1", Port: 2222}); got != "10.0.0.1:2222" {
		t.Fatalf("hostAddressColumn = %q", got)
	}
}
