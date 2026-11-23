package crypto

import (
	"crypto/rand"
	"fmt"
	"io"
)

const SaltLen = 32

func RandomBytes(length int) ([]byte, error) {
	salt := make([]byte, length)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, fmt.Errorf("making salt: %w", err)
	}

	return salt, nil
}
