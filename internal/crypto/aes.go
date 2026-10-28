package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func EncryptGCM(dataBytes []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("starting gcm: %w", err)
	}

	// Number used once
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	cipherBytes := gcm.Seal(nonce, nonce, dataBytes, nil)

	return cipherBytes, nil
}

func DecryptGCM(encriptedBytes []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("starting gcm: %w", err)
	}

	// Get nonce
	nonceSize := gcm.NonceSize()
	if len(encriptedBytes) < nonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("encrypted data is too short or corrupted")
	}

	nonce, actualCipherData := encriptedBytes[:nonceSize], encriptedBytes[nonceSize:]

	dataBytes, err := gcm.Open(nil, nonce, actualCipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("tried desciphering data with KEK: %w", err)
	}

	return dataBytes, nil
}
