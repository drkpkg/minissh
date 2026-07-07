package termius

import (
	"encoding/json"
	"testing"

	"github.com/danieluremix/minissh/internal/model"
)

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestBuildPasswordMatchIsFirstMatchDeterministic(t *testing.T) {
	// Two identities share the same username but have different passwords.
	// The reference script's bug picks whichever is last in iteration
	// order; build() must deterministically pick the first one seen.
	blocks := [][]byte{
		mustJSON(t, map[string]interface{}{"username": "root", "password": "first-pw"}),
		mustJSON(t, map[string]interface{}{"username": "root", "password": "second-pw"}),
		mustJSON(t, map[string]interface{}{"host": "10.0.0.1", "user_name": "root", "connection_type": "ssh", "title": "web-1"}),
	}
	res := build(parseBlocks(blocks))

	if len(res.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d: %+v", len(res.Hosts), res.Hosts)
	}
	if res.Hosts[0].Identity.Kind != model.IdentityPassword {
		t.Fatalf("expected password identity, got %+v", res.Hosts[0].Identity)
	}
	if got := res.Secrets["web-1"]; got != "first-pw" {
		t.Fatalf("expected first-pw (deterministic first match), got %q", got)
	}
}

func TestBuildHostDedupKeysOnHostPortUsername(t *testing.T) {
	// Same host:port, two different users -> both must be kept (this is
	// the reference script's dedup bug: it keyed on host:port alone).
	blocks := [][]byte{
		mustJSON(t, map[string]interface{}{"host": "10.0.0.1", "port": float64(22), "user_name": "alice", "connection_type": "ssh", "title": "web-alice"}),
		mustJSON(t, map[string]interface{}{"host": "10.0.0.1", "port": float64(22), "user_name": "bob", "connection_type": "ssh", "title": "web-bob"}),
		// A true duplicate (same host:port:user) should still be dropped.
		mustJSON(t, map[string]interface{}{"host": "10.0.0.1", "port": float64(22), "user_name": "alice", "connection_type": "ssh", "title": "web-alice-dup"}),
	}
	res := build(parseBlocks(blocks))

	if len(res.Hosts) != 2 {
		t.Fatalf("expected 2 hosts (alice, bob), got %d: %+v", len(res.Hosts), res.Hosts)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("expected 1 skipped duplicate, got %d: %v", len(res.Skipped), res.Skipped)
	}
}

func TestBuildSkipsNonSSHConnectionType(t *testing.T) {
	blocks := [][]byte{
		mustJSON(t, map[string]interface{}{"host": "10.0.0.1", "user_name": "root", "connection_type": "telnet"}),
	}
	res := build(parseBlocks(blocks))
	if len(res.Hosts) != 0 {
		t.Fatalf("expected 0 hosts, got %d", len(res.Hosts))
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(res.Skipped))
	}
}

func TestBuildResolvesKeyIdentityViaKeyID(t *testing.T) {
	blocks := [][]byte{
		mustJSON(t, map[string]interface{}{"id": "k1", "label": "my-key", "private_key": "PEMDATA", "passphrase": "secret"}),
		mustJSON(t, map[string]interface{}{"host": "10.0.0.1", "user_name": "root", "connection_type": "ssh", "title": "web-1", "key_id": "k1"}),
	}
	res := build(parseBlocks(blocks))

	if len(res.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(res.Hosts))
	}
	h := res.Hosts[0]
	if h.Identity.Kind != model.IdentityKey || h.Identity.KeyPath != "my-key" {
		t.Fatalf("expected key identity with label placeholder 'my-key', got %+v", h.Identity)
	}
	km, ok := res.Keys["my-key"]
	if !ok {
		t.Fatal("expected key material for 'my-key'")
	}
	if km.PrivateKeyPEM != "PEMDATA" || km.Passphrase != "secret" {
		t.Fatalf("key material mismatch: %+v", km)
	}
}

func TestParseBlocksSkipsNonJSON(t *testing.T) {
	blocks := [][]byte{
		[]byte("not json"),
		[]byte(""),
		[]byte("{broken"),
	}
	pd := parseBlocks(blocks)
	if len(pd.identities) != 0 || len(pd.connections) != 0 || len(pd.keysByLabel) != 0 {
		t.Fatalf("expected nothing parsed, got %+v", pd)
	}
}
