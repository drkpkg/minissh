// Package termius decrypts and imports hosts directly from a locally
// installed Termius app's own vault, for the case where Termius's own
// export feature isn't available. It reads only the invoking user's own
// local files and their own OS keychain entry (which Termius itself put
// there) — no network access, no writes to Termius's data.
package termius

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zalando/go-keyring"
	ss "github.com/zalando/go-keyring/secret_service"

	"github.com/drkpkg/minissh/internal/importer"
)

const (
	keychainService = "Termius"
	keychainAccount = "localKey"
)

// dbPath returns the location of Termius's local IndexedDB LevelDB
// directory, mirroring where the Termius desktop app stores it per OS.
func dbPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Termius", "IndexedDB", "file__0.indexeddb.leveldb"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}
		return filepath.Join(appData, "Termius", "IndexedDB", "file__0.indexeddb.leveldb"), nil
	default:
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "Termius", "IndexedDB", "file__0.indexeddb.leveldb"), nil
	}
}

// vaultKey retrieves Termius's local vault encryption key from the OS
// keychain — the same entry the Termius desktop app itself reads (via
// keytar, the Electron helper Termius uses for OS credential storage).
func vaultKey() ([]byte, error) {
	encoded, err := keyring.Get(keychainService, keychainAccount)
	if err != nil && runtime.GOOS == "linux" {
		// keytar's Linux backend (libsecret) tags the account under the
		// Secret Service attribute key "account". go-keyring's own Linux
		// backend searches using "username" instead — a real schema
		// mismatch between two libraries targeting the same underlying
		// store, so a miss here doesn't mean the secret isn't there.
		if v, fallbackErr := vaultKeyLinuxFallback(); fallbackErr == nil {
			encoded, err = v, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("reading Termius's local key from the OS keychain: %w (make sure you're logged in to Termius on this machine)", err)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding Termius local key: %w", err)
	}
	return key, nil
}

// vaultKeyLinuxFallback searches the Secret Service directly using the
// "account" attribute keytar actually uses, bypassing go-keyring's
// "username" assumption. Mirrors go-keyring's own secretServiceProvider.Get
// (keyring_unix.go), differing only in the attribute key name.
func vaultKeyLinuxFallback() (string, error) {
	svc, err := ss.NewSecretService()
	if err != nil {
		return "", err
	}

	collection := svc.GetLoginCollection()
	if err := svc.Unlock(collection.Path()); err != nil {
		return "", err
	}

	results, err := svc.SearchItems(collection, map[string]string{
		"service": keychainService,
		"account": keychainAccount,
	})
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", keyring.ErrNotFound
	}

	session, err := svc.OpenSession()
	if err != nil {
		return "", err
	}
	defer func() { _ = svc.Close(session) }()

	if err := svc.Unlock(results[0]); err != nil {
		return "", err
	}
	secret, err := svc.GetSecret(results[0], session.Path())
	if err != nil {
		return "", err
	}
	return string(secret.Value), nil
}

// Import locates the local Termius vault, decrypts it, and returns the
// hosts/groups/keys/secrets it can confidently reconstruct.
func Import() (*importer.Result, error) {
	dir, err := dbPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("termius database not found at %s: %w", dir, err)
	}

	key, err := vaultKey()
	if err != nil {
		return nil, err
	}

	blocks, err := scanEncryptedBlocks(dir)
	if err != nil {
		return nil, err
	}

	stats := &importer.Stats{BlocksFound: len(blocks)}
	var decrypted [][]byte
	for _, b := range blocks {
		if plaintext, ok := decryptBlock(b, key); ok {
			decrypted = append(decrypted, plaintext)
			stats.BlocksDecrypted++
		}
	}

	res := build(parseBlocks(decrypted))
	res.Stats = stats
	return res, nil
}
