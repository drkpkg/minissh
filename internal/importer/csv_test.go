package importer

import (
	"os"
	"strings"
	"testing"

	"github.com/drkpkg/minissh/internal/model"
)

func TestImportCSVFixture(t *testing.T) {
	f, err := os.Open("testdata/termius_export.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, err := ImportCSV(f)
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}

	// legacy-ftp (non-ssh) and no-address-row (missing address) should be skipped.
	if len(res.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d: %+v", len(res.Hosts), res.Hosts)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("expected 2 skipped rows, got %d: %v", len(res.Skipped), res.Skipped)
	}

	byLabel := map[string]model.Host{}
	for _, h := range res.Hosts {
		byLabel[h.Label] = h
	}

	web1, ok := byLabel["web-1"]
	if !ok {
		t.Fatal("expected web-1 host")
	}
	if web1.Address != "10.0.0.1" || web1.Port != 22 || web1.Username != "root" {
		t.Fatalf("web-1 mismatch: %+v", web1)
	}
	if web1.Identity.Kind != model.IdentityKey || web1.Identity.KeyPath != "/home/user/.ssh/id_ed25519" {
		t.Fatalf("web-1 identity mismatch: %+v", web1.Identity)
	}
	if len(web1.Tags) != 2 || web1.Tags[0] != "edge" || web1.Tags[1] != "api" {
		t.Fatalf("web-1 tags mismatch: %+v", web1.Tags)
	}
	if web1.GroupID == "" {
		t.Fatal("expected web-1 to have a group")
	}

	db1, ok := byLabel["db-1"]
	if !ok {
		t.Fatal("expected db-1 host")
	}
	if db1.Port != 2222 {
		t.Fatalf("db-1 port mismatch: %+v", db1)
	}
	if db1.Identity.Kind != model.IdentityPassword {
		t.Fatalf("db-1 expected password identity, got %+v", db1.Identity)
	}
	if got := res.Secrets["db-1"]; got != "hunter2" {
		t.Fatalf("expected db-1 secret hunter2, got %q", got)
	}

	// Row with blank label falls back to using the address as the label.
	if _, ok := byLabel["10.0.0.4"]; !ok {
		t.Fatalf("expected blank-label row to fall back to address as label, got %+v", byLabel)
	}

	// web-1 and db-1 are in different subgroups (web vs db) under the same
	// "prod" parent group, so they should not share a group ID.
	if web1.GroupID == db1.GroupID {
		t.Fatalf("expected distinct subgroups, both got %q", web1.GroupID)
	}

	// Exactly 2 parent groups worth of nesting: prod, prod/web, prod/db = 3 groups.
	if len(res.Groups) != 3 {
		t.Fatalf("expected 3 groups (prod, web, db), got %d: %+v", len(res.Groups), res.Groups)
	}
}

func TestImportCSVEmpty(t *testing.T) {
	res, err := ImportCSV(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ImportCSV empty: %v", err)
	}
	if len(res.Hosts) != 0 {
		t.Fatalf("expected no hosts, got %d", len(res.Hosts))
	}
}
