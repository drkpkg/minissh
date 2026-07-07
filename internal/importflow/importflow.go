// Package importflow persists an importer.Result into minissh's store: the
// one place group/host upserts, private-key file writes, and keychain
// storage happen, shared by every import source (CLI and TUI alike) so
// there's a single implementation of the security-sensitive parts to
// review — never write a plaintext secret, keys always 0600.
package importflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/drkpkg/minissh/internal/importer"
	"github.com/drkpkg/minissh/internal/keychain"
	"github.com/drkpkg/minissh/internal/model"
	"github.com/drkpkg/minissh/internal/store"
)

// Options configures how a Result gets persisted.
type Options struct {
	// KeysDir is where decrypted private key files are written (0600,
	// directory created 0700). Required only if res.Keys is non-nil.
	KeysDir string
	// StoreSecrets controls whether passwords/passphrases are written to
	// the keychain at all. When false, password-auth hosts are still
	// imported — ssh will just prompt for the password normally.
	StoreSecrets bool
	// SetSecret stores a secret under an account name. Defaults to
	// keychain.SetPassword; overridable so callers (tests) can avoid
	// touching the real OS keychain.
	SetSecret func(account, secret string) error
}

// Summary reports what Persist actually did.
type Summary struct {
	HostsImported   int
	GroupsImported  int
	PasswordsStored int
	KeysWritten     int
	Warnings        []string // non-fatal issues (e.g. one host's secret couldn't be stored)
}

// Persist upserts res's hosts/groups into the store, writing any key
// material to disk and any secrets to the keychain along the way. A
// returned error means nothing usable happened (e.g. the store couldn't be
// loaded/saved); per-host problems are recorded in Summary.Warnings instead
// of aborting the whole import.
func Persist(res *importer.Result, opts Options) (Summary, error) {
	setSecret := opts.SetSecret
	if setSecret == nil {
		setSecret = keychain.SetPassword
	}

	s, err := store.Load()
	if err != nil {
		return Summary{}, err
	}

	groupIDRemap := map[string]string{}
	for _, g := range res.Groups {
		parent := groupIDRemap[g.ParentID]
		groupIDRemap[g.ID] = store.UpsertGroup(s, g.Name, parent)
	}

	var summary Summary
	summary.GroupsImported = len(res.Groups)
	writtenKeyPaths := map[string]string{}

	for _, h := range res.Hosts {
		if h.Identity.Kind == model.IdentityKey && res.Keys != nil {
			label := h.Identity.KeyPath // placeholder holding the key's label
			if path, cached := writtenKeyPaths[label]; cached {
				h.Identity.KeyPath = path
			} else {
				km, found := res.Keys[label]
				switch {
				case !found || km.PrivateKeyPEM == "":
					summary.Warnings = append(summary.Warnings, fmt.Sprintf("no key material for %q (host %q); using agent auth instead", label, h.Label))
					h.Identity = model.Identity{Kind: model.IdentityAgent}
				case opts.KeysDir == "":
					summary.Warnings = append(summary.Warnings, fmt.Sprintf("host %q needs a key file but no KeysDir was configured; using agent auth instead", h.Label))
					h.Identity = model.Identity{Kind: model.IdentityAgent}
				default:
					if err := os.MkdirAll(opts.KeysDir, 0o700); err != nil {
						return summary, err
					}
					fileName := sanitizeFileName(label) + "-" + store.NewID() + ".pem"
					path := filepath.Join(opts.KeysDir, fileName)
					if err := os.WriteFile(path, []byte(km.PrivateKeyPEM), 0o600); err != nil {
						return summary, fmt.Errorf("writing key file for %q: %w", label, err)
					}
					writtenKeyPaths[label] = path
					summary.KeysWritten++
					if km.Passphrase != "" && opts.StoreSecrets {
						if err := setSecret("key:"+label, km.Passphrase); err != nil {
							summary.Warnings = append(summary.Warnings, fmt.Sprintf("could not store passphrase for key %q: %v", label, err))
						}
					}
					h.Identity.KeyPath = path
				}
			}
		}

		h.ID = ""
		h.GroupID = groupIDRemap[h.GroupID]
		finalID := store.UpsertHost(s, h)
		summary.HostsImported++

		if opts.StoreSecrets && h.Identity.Kind == model.IdentityPassword {
			if pw, ok := res.Secrets[h.Label]; ok && pw != "" {
				if err := setSecret(finalID, pw); err != nil {
					summary.Warnings = append(summary.Warnings, fmt.Sprintf("could not store password for %q: %v", h.Label, err))
				} else {
					summary.PasswordsStored++
				}
			}
		}
	}

	if err := store.Save(s); err != nil {
		return summary, err
	}
	return summary, nil
}

var unsafeFileNameChars = regexp.MustCompile(`[<>:"/\\|?*]`)

func sanitizeFileName(name string) string {
	return unsafeFileNameChars.ReplaceAllString(name, "_")
}
