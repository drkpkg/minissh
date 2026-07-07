package termius

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

// sealBlock builds a base64 block in the same format decryptBlock expects:
// 1 version byte (4) + 1 reserved byte + 24-byte nonce + secretbox ciphertext.
func sealBlock(t *testing.T, key [32]byte, plaintext string) []byte {
	t.Helper()
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("rand.Read nonce: %v", err)
	}
	ciphertext := secretbox.Seal(nil, []byte(plaintext), &nonce, &key)

	raw := append([]byte{4, 0}, nonce[:]...)
	raw = append(raw, ciphertext...)
	return []byte(base64.StdEncoding.EncodeToString(raw))
}

func TestDecryptBlockRoundTrip(t *testing.T) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatalf("rand.Read key: %v", err)
	}
	block := sealBlock(t, key, `{"host":"10.0.0.1"}`)

	plaintext, ok := decryptBlock(block, key[:])
	if !ok {
		t.Fatal("expected decryptBlock to succeed")
	}
	if string(plaintext) != `{"host":"10.0.0.1"}` {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestDecryptBlockWrongKeyFails(t *testing.T) {
	var key, otherKey [32]byte
	rand.Read(key[:])
	rand.Read(otherKey[:])
	block := sealBlock(t, key, `{"host":"10.0.0.1"}`)

	if _, ok := decryptBlock(block, otherKey[:]); ok {
		t.Fatal("expected decryption to fail with the wrong key")
	}
}

func TestDecryptBlockMalformedInputsDontPanic(t *testing.T) {
	var key [32]byte
	rand.Read(key[:])

	cases := [][]byte{
		nil,
		[]byte(""),
		[]byte("not-base64!!!"),
		[]byte(base64.StdEncoding.EncodeToString([]byte{4})),       // too short
		[]byte(base64.StdEncoding.EncodeToString([]byte{5, 0, 1})), // wrong version byte
	}
	for _, c := range cases {
		if _, ok := decryptBlock(c, key[:]); ok {
			t.Fatalf("expected malformed input to fail decryption: %q", c)
		}
	}
}

func TestScanEncryptedBlocksDedupesAndFiltersExtensions(t *testing.T) {
	var key [32]byte
	rand.Read(key[:])
	block := sealBlock(t, key, `{"a":1}`)

	dir := t.TempDir()
	// Same block appears twice in one .log file, plus once in a .ldb file,
	// plus noise in a file extension we don't scan.
	var logContent []byte
	logContent = append(logContent, "noise "...)
	logContent = append(logContent, block...)
	logContent = append(logContent, " more-noise "...)
	logContent = append(logContent, block...)
	if err := os.WriteFile(filepath.Join(dir, "000001.log"), logContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000002.ldb"), block, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CURRENT"), block, 0o644); err != nil {
		t.Fatal(err)
	}

	blocks, err := scanEncryptedBlocks(dir)
	if err != nil {
		t.Fatalf("scanEncryptedBlocks: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 deduplicated block, got %d: %v", len(blocks), blocks)
	}
}
