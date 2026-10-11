package crypto

import (
	"crypto/rand"
	"fmt"
	"io"
)

// Unique per user
// Salt will be saved in the first 16 bytes of the config
func GenerateSalt() ([]byte, error){
	salt := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil{
		return nil, fmt.Errorf("making salt: %w", err)
	}

	return salt, nil
}