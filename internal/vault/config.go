package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

// Only creates dirs

func (v *Vault) ReadConfigFile() error {
	configPath := filepath.Join(v.Path, "config.bin")
	fileBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	if len(fileBytes) < 16 {
		return fmt.Errorf("unsufficient number of bytes")
	}
	dataBytes := fileBytes[16:]
	decryptedBytes, err := crypto.DecryptGCM(dataBytes, v.masterKey)
	if err != nil {
		return fmt.Errorf("decrypting config file: %w", err)
	}
	defer crypto.CleanBytes(decryptedBytes)

	err = json.Unmarshal(decryptedBytes, &v.Config)
	if err != nil {
		return fmt.Errorf("config file json unmarshal: %w", err)
	}
	return nil
}

func (v *Vault) SaveConfigFile() error {
	jsonData, err := json.Marshal(v.Config)
	if err != nil {
		return fmt.Errorf("encoding json: %w", err)
	}
	defer crypto.CleanBytes(jsonData)

	encryptedBytes, err := crypto.EncryptGCM(jsonData, v.masterKey)
	defer crypto.CleanBytes(encryptedBytes)

	// Append salt to the file
	finalBytes := make([]byte, len(v.salt)+len(encryptedBytes))
	copy(finalBytes, v.salt)
	copy(finalBytes[len(v.salt):], encryptedBytes)
	defer crypto.CleanBytes(finalBytes)

	configPath := filepath.Join(v.Path, "config.bin")
	return os.WriteFile(configPath, finalBytes, 0600)
}
