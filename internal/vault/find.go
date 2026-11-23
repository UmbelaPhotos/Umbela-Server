package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/config"
)

func FindUserFolder(KEK []byte) (string, error) {
	umbelaDir, err := config.UmbelaDir()

	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(umbelaDir)
	if err != nil {
		return "", fmt.Errorf("couldn't read umbela dir: %w", err)
	}

	// Iter for every user directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Ignore files
		}

		folderName := entry.Name()

		folderPath := filepath.Join(umbelaDir, folderName)
		
        _, err = config.Read(folderPath, KEK)
		if err != nil{
			continue
		}

		return folderPath, nil
	}

	return "", ErrUserNotFound
}
