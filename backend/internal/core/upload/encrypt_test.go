package upload

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// multipartFile implements multipart.File for testing using a bytes.Reader
type multipartFile struct{ *bytes.Reader }

func (f multipartFile) Close() error { return nil }

func TestEncryptAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "enc.bin")

	// Prepare input data
	data := []byte("hello encryption")
	mf := multipartFile{bytes.NewReader(data)}

	if err := EncryptAndSave(mf, out); err != nil {
		t.Fatalf("EncryptAndSave failed: %v", err)
	}

	// Verify file exists and has at least IV length
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() <= 16 { // IV is 16 bytes; ciphertext should add more
		t.Fatalf("expected encrypted file to be larger than IV, got %d", info.Size())
	}
}
