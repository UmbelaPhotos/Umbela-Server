package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

func (c *Config) Save(KEK []byte, salt []byte) error {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encoding json: %w", err)
	}
	defer crypto.CleanBytes(jsonData)

	encryptedBytes, err := crypto.EncryptGCM(jsonData, KEK)
	defer crypto.CleanBytes(encryptedBytes)

	// Append salt to the file
	finalBytes := make([]byte, len(salt)+len(encryptedBytes))
	copy(finalBytes, salt)
	copy(finalBytes[len(salt):], encryptedBytes)
	defer crypto.CleanBytes(finalBytes)

	configPath := filepath.Join(c.Vault.DataDir, "config.bin")
	return os.WriteFile(configPath, finalBytes, 0600)
}
