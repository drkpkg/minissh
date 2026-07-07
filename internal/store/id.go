package store

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID returns a short random hex identifier, unique enough for a local
// single-user host list (not a distributed ID scheme).
func NewID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing means the system RNG is broken
	}
	return hex.EncodeToString(b)
}
