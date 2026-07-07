package sources

import "testing"

func TestAllHasUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range All {
		if s.ID == "" {
			t.Fatalf("source %q has empty ID", s.Name)
		}
		if seen[s.ID] {
			t.Fatalf("duplicate source ID %q", s.ID)
		}
		seen[s.ID] = true
		if s.Run == nil {
			t.Fatalf("source %q has nil Run", s.ID)
		}
	}
}

func TestByID(t *testing.T) {
	if _, ok := ByID("csv"); !ok {
		t.Fatal("expected csv source to be registered")
	}
	if _, ok := ByID("termius-live"); !ok {
		t.Fatal("expected termius-live source to be registered")
	}
	if _, ok := ByID("does-not-exist"); ok {
		t.Fatal("expected lookup of unknown ID to fail")
	}
}

func TestFileBasedSourcesRequireFile(t *testing.T) {
	for _, id := range []string{"csv", "json", "sshconfig"} {
		s, ok := ByID(id)
		if !ok {
			t.Fatalf("expected source %q to be registered", id)
		}
		if !s.RequiresFile {
			t.Fatalf("expected %q to require a file", id)
		}
	}
	s, ok := ByID("termius-live")
	if !ok {
		t.Fatal("expected termius-live to be registered")
	}
	if s.RequiresFile {
		t.Fatal("expected termius-live not to require a file")
	}
}

func TestCSVSourceRunsAgainstFixture(t *testing.T) {
	s, _ := ByID("csv")
	res, err := s.Run("../importer/testdata/termius_export.csv")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Hosts) == 0 {
		t.Fatal("expected at least one host from the fixture")
	}
}
