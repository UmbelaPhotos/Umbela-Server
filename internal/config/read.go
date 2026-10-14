package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

// Only creates dirs

func (c *Config) ReadConfigFile(masterKey []byte) error {
	configPath := filepath.Join(c.UserDir, "config.bin")
	fileBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	if len(fileBytes) < 16 {
		return fmt.Errorf("unsufficient number of bytes")
	}
	dataBytes := fileBytes[16:]
	decryptedBytes, err := crypto.DecryptGCM(dataBytes, masterKey)
	if err != nil {
		return fmt.Errorf("decrypting config file: %w", err)
	}
	defer crypto.CleanBytes(decryptedBytes)

	err = json.Unmarshal(decryptedBytes, c)
	if err != nil {
		return fmt.Errorf("config file json unmarshal: %w", err)
	}
	return nil
}
