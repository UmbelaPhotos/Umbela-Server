package config

import (
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
)

type Config struct {
	Server ServerConfig `json:"server"`
	Vault  VaultConfig  `json:"vault"`
}

type ServerConfig struct {
	APIPort    int    `json:"api_port"`
	TorEnabled bool   `json:"tor_enabled"`
	OneHopTor  bool   `json:"one_hop_enabled"`
	TorEd25519 []byte `json:"tor_ed25519,omitempty"`
	WorkerCount int `json:"worker_count"`
}

type VaultConfig struct {
	DEK []byte `json:"encrypted_dek"`
	DataDir      string `json:"data_dir"`
	ImageDir     string `json:"image_dir"`
	ThumbsDir    string `json:"thumbs_dir"`
	DBPath       string `json:"db_path"`
	ThumbWidth   int    `json:"thumb_width"`
	ThumbHeight  int    `json:"thumb_height"`
	ZipDepth	 int	`json:"zip_depth"`
}

// DefaultConfig returns the configuration with the user home paths.
func DefaultConfig(userPath string) (*Config, error) {
	if err := os.MkdirAll(userPath, 0700); err != nil {
		return nil, fmt.Errorf("creating user dir: %w", err)
	}

	info, err := os.Stat(userPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("path: %s exists but is not valid", userPath)
	}

	DEK, err := crypto.RandomBytes(32)
	if err!=nil{
		return nil, fmt.Errorf("creating DEK: %w", err)
	}

	return &Config{
		Vault: VaultConfig{
			DEK: 		 DEK,
			DataDir:     userPath,
			ImageDir:    filepath.Join(userPath, "images"),
			ThumbsDir:   filepath.Join(userPath, "thumbs"),
			DBPath:      filepath.Join(userPath, "umbela.db"),
			ZipDepth:    3,
		},
		Server: ServerConfig{
			APIPort:     41110,
			TorEnabled:  true,
			OneHopTor:   true,
			TorEd25519:  nil,
			WorkerCount: 4,
		},
	}, nil
}
