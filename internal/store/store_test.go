package store

import (
	"testing"
	"time"

	"github.com/drkpkg/minissh/internal/model"
)

func TestLoadMissingFileReturnsEmptyStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Hosts) != 0 || len(s.Groups) != 0 {
		t.Fatalf("expected empty store, got %+v", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := &model.Store{
		Groups: []model.Group{{ID: "g1", Name: "prod"}},
		Hosts: []model.Host{
			{ID: "h1", Label: "web-1", Address: "10.0.0.1", Port: 22, Username: "root", GroupID: "g1"},
		},
	}
	if err := Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Hosts) != 1 || loaded.Hosts[0].Label != "web-1" {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
	if len(loaded.Groups) != 1 || loaded.Groups[0].Name != "prod" {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}

func TestUpsertGroupDeduplicatesByNameAndParent(t *testing.T) {
	s := &model.Store{}
	id1 := UpsertGroup(s, "prod", "")
	id2 := UpsertGroup(s, "prod", "")
	if id1 != id2 {
		t.Fatalf("expected same group id, got %q and %q", id1, id2)
	}
	if len(s.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(s.Groups))
	}

	// Same name, different parent -> distinct group.
	id3 := UpsertGroup(s, "prod", id1)
	if id3 == id1 {
		t.Fatalf("expected distinct group id for different parent")
	}
	if len(s.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(s.Groups))
	}
}

func TestUpsertGroupEmptyNameReturnsEmptyID(t *testing.T) {
	s := &model.Store{}
	if id := UpsertGroup(s, "", ""); id != "" {
		t.Fatalf("expected empty id for empty name, got %q", id)
	}
	if len(s.Groups) != 0 {
		t.Fatalf("expected no group created, got %d", len(s.Groups))
	}
}

func TestUpsertHostInsertsNew(t *testing.T) {
	s := &model.Store{}
	id := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1"})
	if id == "" {
		t.Fatal("expected generated id")
	}
	if len(s.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(s.Hosts))
	}
}

func TestUpsertHostUpdatesExistingByLabelAndAddress(t *testing.T) {
	s := &model.Store{}
	id := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1", Username: "alice"})

	id2 := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1", Username: "bob"})
	if id2 != id {
		t.Fatalf("expected same id on update, got %q vs %q", id2, id)
	}
	if len(s.Hosts) != 1 {
		t.Fatalf("expected still 1 host, got %d", len(s.Hosts))
	}
	if s.Hosts[0].Username != "bob" {
		t.Fatalf("expected host updated to bob, got %q", s.Hosts[0].Username)
	}
}

func TestFindAndDeleteHost(t *testing.T) {
	s := &model.Store{}
	UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1"})

	if idx := FindHostIndex(s, "web-1"); idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
	if idx := FindHostIndex(s, "missing"); idx != -1 {
		t.Fatalf("expected -1 for missing host, got %d", idx)
	}

	if !DeleteHost(s, "web-1") {
		t.Fatal("expected DeleteHost to succeed")
	}
	if len(s.Hosts) != 0 {
		t.Fatalf("expected host removed, got %d remaining", len(s.Hosts))
	}
	if DeleteHost(s, "web-1") {
		t.Fatal("expected DeleteHost to report false for already-removed host")
	}
}

func TestDeleteHostByID(t *testing.T) {
	s := &model.Store{}
	id := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1"})

	if !DeleteHostByID(s, id) {
		t.Fatal("expected DeleteHostByID to succeed")
	}
	if len(s.Hosts) != 0 {
		t.Fatalf("expected host removed, got %d remaining", len(s.Hosts))
	}
	if DeleteHostByID(s, id) {
		t.Fatal("expected DeleteHostByID to report false for already-removed host")
	}
}

func TestSetFavorite(t *testing.T) {
	s := &model.Store{}
	id := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1"})

	if !SetFavorite(s, id, true) {
		t.Fatal("expected SetFavorite to succeed")
	}
	if !s.Hosts[0].Favorite {
		t.Fatal("expected host marked favorite")
	}

	if !SetFavorite(s, id, false) {
		t.Fatal("expected SetFavorite to succeed")
	}
	if s.Hosts[0].Favorite {
		t.Fatal("expected host unmarked favorite")
	}

	if SetFavorite(s, "missing", true) {
		t.Fatal("expected SetFavorite to report false for unknown id")
	}
}

func TestRecordConnected(t *testing.T) {
	s := &model.Store{}
	id := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1"})

	if !s.Hosts[0].LastConnectedAt.IsZero() {
		t.Fatal("expected zero LastConnectedAt before any connection")
	}

	now := time.Now().Truncate(time.Second)
	if !RecordConnected(s, id, now) {
		t.Fatal("expected RecordConnected to succeed")
	}
	if !s.Hosts[0].LastConnectedAt.Equal(now) {
		t.Fatalf("expected LastConnectedAt %v, got %v", now, s.Hosts[0].LastConnectedAt)
	}

	if RecordConnected(s, "missing", now) {
		t.Fatal("expected RecordConnected to report false for unknown id")
	}
}

func TestFavoriteAndLastConnectedSurviveSaveLoad(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := &model.Store{}
	id := UpsertHost(s, model.Host{Label: "web-1", Address: "10.0.0.1"})
	SetFavorite(s, id, true)
	now := time.Now().Truncate(time.Second)
	RecordConnected(s, id, now)
	if err := Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Hosts[0].Favorite {
		t.Fatal("expected Favorite to survive round trip")
	}
	if !loaded.Hosts[0].LastConnectedAt.Equal(now) {
		t.Fatalf("expected LastConnectedAt %v, got %v", now, loaded.Hosts[0].LastConnectedAt)
	}
}
