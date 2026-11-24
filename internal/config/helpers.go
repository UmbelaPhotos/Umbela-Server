package config

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"umbela-server/internal/crypto"
)

func UmbelaDir() (string, error) {
	if d := os.Getenv("UMBELA_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		u, uerr := user.Current()
		if uerr != nil || u.HomeDir == "" {
			return "", fmt.Errorf("cannot determine home dir: set UMBELA_HOME")
		}
		home = u.HomeDir
	}
	umbelaPath:=filepath.Join(home, ".umbela")
	err = os.MkdirAll(umbelaPath, 0700)
	if err!=nil{
		return "", fmt.Errorf("creating umbela dir: %w", err)
	}
	return umbelaPath, nil
}


func GetSalt() ([]byte, error){
	dataDir,err := UmbelaDir()
	if err!=nil{
		return nil, err
	}
	saltPath := filepath.Join(dataDir, ".salt")
	fileBytes, err := os.ReadFile(saltPath)
	if errors.Is(err, os.ErrNotExist) {
		fileBytes, err = CopyOrCreateSalt()
	}
	if err!=nil{
		return nil, err
	}
	if len(fileBytes)!=crypto.SaltLen{
		return nil, fmt.Errorf("incorrect salt lenght")
	}
	return fileBytes, nil
}

func CopyOrCreateSalt() ([]byte, error){
	umbelaDir, err := UmbelaDir()
	if err!=nil{
		return nil, fmt.Errorf("umbela dir: %w", err)
	}
	umbelaSaltPath := filepath.Join(umbelaDir, ".salt")

	var salt []byte

	entries, err := os.ReadDir(umbelaDir)
	if err != nil {
		return nil, fmt.Errorf("couldn't read umbela dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Ignore files
		}
		saltPath := filepath.Join(umbelaDir, entry.Name(), ".salt")
		salt, err = os.ReadFile(saltPath)
		if err != nil{
			continue
		}
		if len(salt)!=crypto.SaltLen{
			return nil, fmt.Errorf("incorrect salt lenght")
		}
		break
	}
	if salt==nil || len(salt)!=crypto.SaltLen{
		salt, err=crypto.RandomBytes(crypto.SaltLen)
		if err!=nil{
			return nil, fmt.Errorf("creating salt: %w", err)
		}
	} 
	os.WriteFile(umbelaSaltPath, salt, 0600)
	return salt, nil 
}