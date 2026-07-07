// Package model defines the core host/group data structures used across minissh.
package model

import "time"

// IdentityKind describes how a Host authenticates.
type IdentityKind string

const (
	IdentityAgent    IdentityKind = "agent"
	IdentityKey      IdentityKind = "key"
	IdentityPassword IdentityKind = "password"
)

// Identity describes how to authenticate to a Host. The password itself is
// never stored here — it lives in the OS keychain, looked up by Host.ID.
type Identity struct {
	Kind    IdentityKind `json:"kind"`
	KeyPath string       `json:"keyPath,omitempty"`
}

// Host is a single SSH connection target.
type Host struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	GroupID  string   `json:"groupId,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Identity Identity `json:"identity"`

	Favorite bool `json:"favorite,omitempty"`
	// LastConnectedAt is the zero value (check with IsZero) if never
	// connected. Note: encoding/json's omitempty doesn't apply to struct
	// types like time.Time, so a zero value still serializes explicitly —
	// harmless, just not omitted.
	LastConnectedAt time.Time `json:"lastConnectedAt"`
	Notes           string    `json:"notes,omitempty"`
}

// Group is a (possibly nested) folder of Hosts.
type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
}

// Store is the full persisted state: every Host and Group minissh knows about.
type Store struct {
	Hosts  []Host  `json:"hosts"`
	Groups []Group `json:"groups"`
}
