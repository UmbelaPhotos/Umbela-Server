package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

func (c *Config) Save(KEK []byte) error {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encoding json: %w", err)
	}
	defer crypto.CleanBytes(jsonData)

	encryptedBytes, err := crypto.EncryptGCM(jsonData, KEK)
	if err != nil {
		return fmt.Errorf("encrypting config: %w", err)
	}
	defer crypto.CleanBytes(encryptedBytes)

	configPath := filepath.Join(c.Vault.DataDir, "config.bin")
	return os.WriteFile(configPath, encryptedBytes, 0600)
}
