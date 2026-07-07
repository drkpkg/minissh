package importer

import (
	"os"
	"strings"
	"testing"

	"github.com/drkpkg/minissh/internal/model"
)

func TestImportSSHConfigFixture(t *testing.T) {
	f, err := os.Open("testdata/ssh_config")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, err := ImportSSHConfig(f)
	if err != nil {
		t.Fatalf("ImportSSHConfig: %v", err)
	}

	// "Host *" and "Host bastion-*" are wildcard patterns and should be skipped.
	if len(res.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d: %+v", len(res.Hosts), res.Hosts)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("expected 2 skipped wildcard entries, got %d: %v", len(res.Skipped), res.Skipped)
	}

	byLabel := map[string]model.Host{}
	for _, h := range res.Hosts {
		byLabel[h.Label] = h
	}

	web1, ok := byLabel["web-1"]
	if !ok {
		t.Fatal("expected web-1 host")
	}
	if web1.Address != "10.0.0.1" || web1.Port != 2222 || web1.Username != "root" {
		t.Fatalf("web-1 mismatch: %+v", web1)
	}
	if web1.Identity.Kind != model.IdentityKey || !strings.HasSuffix(web1.Identity.KeyPath, "/.ssh/id_ed25519") {
		t.Fatalf("web-1 identity mismatch: %+v", web1.Identity)
	}

	db1, ok := byLabel["db-1"]
	if !ok {
		t.Fatal("expected db-1 host")
	}
	if db1.Port != 22 {
		t.Fatalf("expected default port 22, got %d", db1.Port)
	}
	if db1.Identity.Kind != model.IdentityAgent {
		t.Fatalf("expected agent identity by default, got %+v", db1.Identity)
	}
}

func TestImportSSHConfigEmpty(t *testing.T) {
	res, err := ImportSSHConfig(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ImportSSHConfig empty: %v", err)
	}
	if len(res.Hosts) != 0 {
		t.Fatalf("expected no hosts, got %d", len(res.Hosts))
	}
}

func TestImportSSHConfigHostAliasFallsBackToAddress(t *testing.T) {
	res, err := ImportSSHConfig(strings.NewReader("Host plainbox\n  User bob\n"))
	if err != nil {
		t.Fatalf("ImportSSHConfig: %v", err)
	}
	if len(res.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(res.Hosts))
	}
	if res.Hosts[0].Address != "plainbox" {
		t.Fatalf("expected address to fall back to alias, got %+v", res.Hosts[0])
	}
}
