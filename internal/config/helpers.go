package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetExecutableAndUmbelaPath()(string, string, error){
	exePath, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("No executable dir: %w", err)
	}
	
	exeDir:=filepath.Dir(exePath)
	umbelaDirectory := filepath.Join(exeDir, ".umbela")

	return exeDir, umbelaDirectory, nil 
}