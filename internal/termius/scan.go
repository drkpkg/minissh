package termius

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/nacl/secretbox"
)

// encryptedBlockPattern matches base64 blobs starting with "BA" (which
// decodes to the 0x04 version-byte marker checked in decryptBlock) — the
// same heuristic used to find Termius's encrypted records embedded in raw
// LevelDB .log/.ldb files without a real LevelDB parse.
var encryptedBlockPattern = regexp.MustCompile(`BA[A-Za-z0-9+/=]{30,}`)

// scanEncryptedBlocks reads every *.log and *.ldb file in dir and returns
// the deduplicated set of candidate encrypted blocks found in them.
func scanEncryptedBlocks(dir string) ([][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var all []byte
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".ldb")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		all = append(all, data...)
		// Separator so a match can never span two files' contents — e.g. a
		// block ending exactly at one file's end with another starting
		// right at the next file's start would otherwise merge into one
		// bogus, unmatched candidate. '\n' isn't in the base64 charset.
		all = append(all, '\n')
	}

	matches := encryptedBlockPattern.FindAll(all, -1)
	seen := make(map[string]bool, len(matches))
	var blocks [][]byte
	for _, m := range matches {
		k := string(m)
		if seen[k] {
			continue
		}
		seen[k] = true
		blocks = append(blocks, m)
	}
	return blocks, nil
}

// decryptBlock attempts to decrypt a single candidate block. Format:
// 1 version byte (must be 4) + 1 reserved byte + 24-byte nonce + NaCl
// secretbox ciphertext. Returns ok==false for anything that doesn't
// authenticate — expected for most candidates, since the regex scan over
// raw bytes produces false positives at record boundaries.
func decryptBlock(b64 []byte, key []byte) ([]byte, bool) {
	if len(key) != 32 {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil {
		return nil, false
	}
	if len(data) < 26 || data[0] != 4 {
		return nil, false
	}

	var nonce [24]byte
	copy(nonce[:], data[2:26])
	var keyArr [32]byte
	copy(keyArr[:], key)

	return secretbox.Open(nil, data[26:], &nonce, &keyArr)
}
