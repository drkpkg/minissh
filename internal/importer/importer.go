// Package importer converts host lists from external tools (Termius CSV/JSON
// export, ~/.ssh/config) into minissh's model.
package importer

import "github.com/danieluremix/minissh/internal/model"

// Result is the outcome of parsing an external host list.
type Result struct {
	Hosts   []model.Host
	Groups  []model.Group
	Secrets map[string]string      // host Label -> plaintext password, caller stores in keychain
	Keys    map[string]KeyMaterial // key label -> decrypted key material; nil unless the source decrypts real key bytes (e.g. termius-live)
	Skipped []string               // human-readable reasons rows/entries were skipped
	Stats   *Stats                 // nil unless the source reports scan/decrypt stats (e.g. termius-live)
}

// KeyMaterial is a decrypted SSH private key and its optional passphrase,
// for sources that recover real key bytes rather than pointing at a file
// that already exists on disk.
type KeyMaterial struct {
	PrivateKeyPEM string
	Passphrase    string
}

// Stats reports how much of a source could be scanned/decrypted, for
// sources where that isn't a given (e.g. termius-live's best-effort scan).
type Stats struct {
	BlocksFound     int
	BlocksDecrypted int
}
