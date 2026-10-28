package crypto

import (
	"crypto/rand"
	"fmt"
	"io"
)

// Unique per user
// Salt will be saved in the first 16 bytes of the config
func RandomBytes(length int) ([]byte, error) {
	salt := make([]byte, length)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, fmt.Errorf("making salt: %w", err)
	}

	return salt, nil
}
