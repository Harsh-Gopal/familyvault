package upload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"mime/multipart"
	"os"
)

var encryptionKey = []byte("0123456789abcdef0123456789abcdef")

// GetKey returns the AES-256 key used for encryption/decryption.
func GetKey() []byte {
	return encryptionKey
}

// EncryptAndSave encrypts the input stream using AES-256-CTR and writes the
// result to outputPath. The first 16 bytes of the file contain the random IV.
func EncryptAndSave(input multipart.File, outputPath string) error {
	// Create destination file (overwrite if exists)
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return err
	}
	if _, err := out.Write(iv); err != nil {
		return err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return err
	}
	stream := cipher.NewCTR(block, iv)
	writer := &cipher.StreamWriter{S: stream, W: out}

	// Stream copy in chunks via io.Copy
	if _, err := io.Copy(writer, input); err != nil {
		return err
	}
	return nil
}
