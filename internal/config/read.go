package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

// Only creates dirs
func Read(dataDir string, KEK []byte) (*Config, error) {
	configPath := filepath.Join(dataDir, "config.bin")
	fileBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// 16 bytes of salt + 12 nonce  
	if len(fileBytes) < 28 {
		return nil, fmt.Errorf("unsufficient number of bytes")
	}
	dataBytes := fileBytes[16:]
	decryptedBytes, err := crypto.DecryptGCM(dataBytes, KEK)
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
