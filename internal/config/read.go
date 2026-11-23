package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

func Read(dataDir string, KEK []byte) (*Config, error) {
	configPath := filepath.Join(dataDir, "config.bin")
	fileBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if len(fileBytes) < 12 {
		return nil, fmt.Errorf("unsufficient number of bytes")
	}
	decryptedBytes, err := crypto.DecryptGCM(fileBytes, KEK)
	if err != nil {
		return nil, fmt.Errorf("decrypting config file: %w", err)
	}
	defer crypto.CleanBytes(decryptedBytes)

	c:=&Config{}
	err = json.Unmarshal(decryptedBytes, c)
	if err != nil {
		return nil, fmt.Errorf("config file json unmarshal: %w", err)
	}
	return c, nil
}
