package download

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
	"os"

	up "familyvault/internal/core/upload"
)

// DecryptAndStream opens the encrypted file at inputPath, reads the first 16 bytes
// as IV, and streams AES-256-CTR decrypted contents to writer.
func DecryptAndStream(inputPath string, writer io.Writer) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer in.Close()

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(in, iv); err != nil {
		return err
	}
	block, err := aes.NewCipher(up.GetKey())
	if err != nil {
		return err
	}
	stream := cipher.NewCTR(block, iv)
	reader := &cipher.StreamReader{S: stream, R: in}
	_, err = io.Copy(writer, reader)
	return err
}
