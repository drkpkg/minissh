package importflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danieluremix/minissh/internal/importer"
	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/store"
)

func fakeSecretStore() (func(string, string) error, map[string]string) {
	secrets := map[string]string{}
	return func(account, secret string) error {
		secrets[account] = secret
		return nil
	}, secrets
}

func TestPersistHostsAndGroups(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setSecret, _ := fakeSecretStore()

	res := &importer.Result{
		Groups: []model.Group{{ID: "g1", Name: "prod"}},
		Hosts: []model.Host{
			{Label: "web-1", Address: "10.0.0.1", Port: 22, GroupID: "g1", Identity: model.Identity{Kind: model.IdentityAgent}},
		},
	}

	summary, err := Persist(res, Options{SetSecret: setSecret})
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if summary.HostsImported != 1 || summary.GroupsImported != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if len(s.Hosts) != 1 || s.Hosts[0].Label != "web-1" {
		t.Fatalf("expected host persisted, got %+v", s.Hosts)
	}
	if len(s.Groups) != 1 || s.Groups[0].Name != "prod" {
		t.Fatalf("expected group persisted, got %+v", s.Groups)
	}
	if s.Hosts[0].GroupID != s.Groups[0].ID {
		t.Fatalf("expected host's GroupID to be remapped to the real group ID, got host=%q group=%q", s.Hosts[0].GroupID, s.Groups[0].ID)
	}
}

func TestPersistStoresPasswordWhenEnabled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setSecret, secrets := fakeSecretStore()

	res := &importer.Result{
		Hosts:   []model.Host{{Label: "db-1", Address: "10.0.0.2", Identity: model.Identity{Kind: model.IdentityPassword}}},
		Secrets: map[string]string{"db-1": "hunter2"},
	}

	summary, err := Persist(res, Options{StoreSecrets: true, SetSecret: setSecret})
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if summary.PasswordsStored != 1 {
		t.Fatalf("expected 1 password stored, got %+v", summary)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret recorded, got %v", secrets)
	}
}

func TestPersistSkipsPasswordWhenDisabled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setSecret, secrets := fakeSecretStore()

	res := &importer.Result{
		Hosts:   []model.Host{{Label: "db-1", Address: "10.0.0.2", Identity: model.Identity{Kind: model.IdentityPassword}}},
		Secrets: map[string]string{"db-1": "hunter2"},
	}

	summary, err := Persist(res, Options{StoreSecrets: false, SetSecret: setSecret})
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if summary.PasswordsStored != 0 || len(secrets) != 0 {
		t.Fatalf("expected no password stored, got summary=%+v secrets=%v", summary, secrets)
	}
}

func TestPersistWritesKeyFileWithRestrictivePermissions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setSecret, secrets := fakeSecretStore()
	keysDir := filepath.Join(t.TempDir(), "keys")

	res := &importer.Result{
		Hosts: []model.Host{
			{Label: "web-1", Address: "10.0.0.1", Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "my-key"}},
		},
		Keys: map[string]importer.KeyMaterial{
			"my-key": {PrivateKeyPEM: "PEMDATA", Passphrase: "keysecret"},
		},
	}

	summary, err := Persist(res, Options{KeysDir: keysDir, StoreSecrets: true, SetSecret: setSecret})
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if summary.KeysWritten != 1 {
		t.Fatalf("expected 1 key written, got %+v", summary)
	}
	if secrets["key:my-key"] != "keysecret" {
		t.Fatalf("expected key passphrase stored under 'key:my-key', got %v", secrets)
	}

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if len(s.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(s.Hosts))
	}
	keyPath := s.Hosts[0].Identity.KeyPath
	if keyPath == "" || keyPath == "my-key" {
		t.Fatalf("expected KeyPath to be rewritten to a real file path, got %q", keyPath)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected key file mode 0600, got %o", perm)
	}

	dirInfo, err := os.Stat(keysDir)
	if err != nil {
		t.Fatalf("stat keys dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("expected keys dir mode 0700, got %o", perm)
	}
}

func TestPersistSharesKeyFileAcrossHosts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setSecret, _ := fakeSecretStore()
	keysDir := t.TempDir()

	res := &importer.Result{
		Hosts: []model.Host{
			{Label: "web-1", Address: "10.0.0.1", Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "shared-key"}},
			{Label: "web-2", Address: "10.0.0.2", Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "shared-key"}},
		},
		Keys: map[string]importer.KeyMaterial{
			"shared-key": {PrivateKeyPEM: "PEMDATA"},
		},
	}

	summary, err := Persist(res, Options{KeysDir: keysDir, SetSecret: setSecret})
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if summary.KeysWritten != 1 {
		t.Fatalf("expected key written exactly once (shared), got %d", summary.KeysWritten)
	}

	s, _ := store.Load()
	if s.Hosts[0].Identity.KeyPath != s.Hosts[1].Identity.KeyPath {
		t.Fatalf("expected both hosts to share the same key path, got %q and %q", s.Hosts[0].Identity.KeyPath, s.Hosts[1].Identity.KeyPath)
	}
}

func TestPersistFallsBackToAgentWhenKeyMaterialMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setSecret, _ := fakeSecretStore()

	res := &importer.Result{
		Hosts: []model.Host{
			{Label: "web-1", Address: "10.0.0.1", Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "missing-key"}},
		},
		Keys: map[string]importer.KeyMaterial{},
	}

	summary, err := Persist(res, Options{KeysDir: t.TempDir(), SetSecret: setSecret})
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if len(summary.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", summary.Warnings)
	}

	s, _ := store.Load()
	if s.Hosts[0].Identity.Kind != model.IdentityAgent {
		t.Fatalf("expected fallback to agent auth, got %+v", s.Hosts[0].Identity)
	}
}
