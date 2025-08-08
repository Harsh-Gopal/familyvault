package download

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	up "familyvault/internal/core/upload"
)

// multipartFile implements multipart.File behavior needed by EncryptAndSave
type multipartFile struct{ *bytes.Reader }

func (f multipartFile) Close() error { return nil }

// Ensure we can decrypt what we encrypt
func TestDecryptAndStream_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "cipher.bin")
	plaintext := []byte("roundtrip test content")

	// Encrypt
	mf := multipartFile{bytes.NewReader(plaintext)}
	if err := up.EncryptAndSave(mf, inPath); err != nil {
		t.Fatalf("EncryptAndSave: %v", err)
	}

	// Decrypt
	var buf bytes.Buffer
	if err := DecryptAndStream(inPath, &buf); err != nil {
		t.Fatalf("DecryptAndStream: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), plaintext) {
		t.Fatalf("plaintext mismatch: got %q", buf.String())
	}

	// Ensure file exists and has IV + ciphertext
	info, err := os.Stat(inPath)
	if err != nil || info.Size() <= 16 {
		t.Fatalf("cipher file invalid")
	}
}
