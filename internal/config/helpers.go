package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
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
	return filepath.Join(home, ".umbela"), nil
}