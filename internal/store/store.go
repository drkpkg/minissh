// Package store persists the minissh host/group list to a local JSON file.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/drkpkg/minissh/internal/model"
)

// Path returns the location of the hosts JSON file, honoring XDG_CONFIG_HOME.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "minissh", "hosts.json"), nil
}

// KeysDir returns the directory minissh writes decrypted/imported private
// key files into.
func KeysDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "minissh", "keys"), nil
}

// Load reads the store from disk, returning an empty Store if the file
// doesn't exist yet.
func Load() (*model.Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &model.Store{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s model.Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes the store to disk atomically (write to a temp file, then rename).
func Save(s *model.Store) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// UpsertGroup finds a group by name+parent, creating it if absent, and
// returns its ID.
func UpsertGroup(s *model.Store, name, parentID string) string {
	if name == "" {
		return ""
	}
	for i := range s.Groups {
		if s.Groups[i].Name == name && s.Groups[i].ParentID == parentID {
			return s.Groups[i].ID
		}
	}
	g := model.Group{ID: NewID(), Name: name, ParentID: parentID}
	s.Groups = append(s.Groups, g)
	return g.ID
}

// UpsertHost inserts h, or updates an existing host matched by Label+Address
// in place, preserving its ID. Returns the host's final ID.
func UpsertHost(s *model.Store, h model.Host) string {
	for i := range s.Hosts {
		if s.Hosts[i].Label == h.Label && s.Hosts[i].Address == h.Address {
			id := s.Hosts[i].ID
			h.ID = id
			s.Hosts[i] = h
			return id
		}
	}
	if h.ID == "" {
		h.ID = NewID()
	}
	s.Hosts = append(s.Hosts, h)
	return h.ID
}

// FindHostIndex returns the index of the host with the given label, or -1
// if none exists.
func FindHostIndex(s *model.Store, label string) int {
	for i := range s.Hosts {
		if s.Hosts[i].Label == label {
			return i
		}
	}
	return -1
}

// DeleteHost removes the host with the given label. Returns false if no
// such host existed.
func DeleteHost(s *model.Store, label string) bool {
	i := FindHostIndex(s, label)
	if i == -1 {
		return false
	}
	s.Hosts = append(s.Hosts[:i], s.Hosts[i+1:]...)
	return true
}

// DeleteHostByID removes the host with the given ID. Returns false if no
// such host existed.
func DeleteHostByID(s *model.Store, id string) bool {
	for i := range s.Hosts {
		if s.Hosts[i].ID == id {
			s.Hosts = append(s.Hosts[:i], s.Hosts[i+1:]...)
			return true
		}
	}
	return false
}

// SetFavorite sets the favorite flag on the host with the given ID. Returns
// false if no such host exists.
func SetFavorite(s *model.Store, hostID string, favorite bool) bool {
	for i := range s.Hosts {
		if s.Hosts[i].ID == hostID {
			s.Hosts[i].Favorite = favorite
			return true
		}
	}
	return false
}

// RecordConnected stamps the host's LastConnectedAt. Returns false if no
// such host exists.
func RecordConnected(s *model.Store, hostID string, at time.Time) bool {
	for i := range s.Hosts {
		if s.Hosts[i].ID == hostID {
			s.Hosts[i].LastConnectedAt = at
			return true
		}
	}
	return false
}
