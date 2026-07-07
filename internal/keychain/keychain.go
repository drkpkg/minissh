// Package keychain stores host passwords in the OS credential store (Secret
// Service/GNOME Keyring on Linux, Keychain on macOS, Credential Manager on
// Windows) rather than in minissh's plaintext hosts.json.
package keychain

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "minissh"

// SetPassword stores password for the given host ID, overwriting any
// existing entry.
func SetPassword(hostID, password string) error {
	return keyring.Set(service, hostID, password)
}

// GetPassword retrieves the password for the given host ID. It returns
// ("", nil) if no password is stored, rather than an error, so callers can
// fall back to an interactive prompt without special-casing "not found".
func GetPassword(hostID string) (string, error) {
	pw, err := keyring.Get(service, hostID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return pw, err
}

// DeletePassword removes the stored password for the given host ID, if any.
func DeletePassword(hostID string) error {
	err := keyring.Delete(service, hostID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
