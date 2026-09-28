package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
	"umbela-server/internal/models"
)

func FindUserFolder(username string) (string, []byte, error) {
	_, umbelaDir, err := models.GetExecutableAndUmbelaPath()

	if err != nil {
		return "", nil, err
	}

	entries, err := os.ReadDir(umbelaDir)
	if err != nil {
		return "", nil, fmt.Errorf("couldn't read umbela dir: %w", err)
	}

	// Iter for every user directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Ignore files
		}

		folderHashedName := entry.Name()
		configPath := filepath.Join(umbelaDir, folderHashedName, "config.enc")

		file, err := os.Open(configPath)
		if err != nil {
			// No config
			continue
		}

		salt := make([]byte, 16)
		_, err = file.Read(salt)
		file.Close()
		if err != nil {
			continue
		}

		plaintext := crypto.HashText(username, salt)

		if plaintext == folderHashedName {
			return filepath.Join(umbelaDir, folderHashedName), salt, nil
		}
	}

	return "", nil, ErrUserNotFound
}
