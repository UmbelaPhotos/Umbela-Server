package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir	  string `json:"data_dir"`
	ImageDir  string `json:"image_dir"`
	ThumbsDir string `json:"thumbs_dir"`
	DBPath    string `json:"db_path"`
	ZipDepth  int	 `json:"zip_depth"`

	APIPort    int    `json:"api_port"`
	TorEnabled bool   `json:"tor_enabled"`
	OneHopTor  bool   `json:"one_hop_enabled"`
	TorEd25519 []byte `json:"tor_ed25519,omitempty"`

	ThumbWidth  int `json:"thumb_width"`
	ThumbHeight int `json:"thumb_height"`
	WorkerCount int `json:"worker_count"`
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

	return &Config{
		DataDir:	 userPath,
		ImageDir:    filepath.Join(userPath, "images"),
		ThumbsDir:   filepath.Join(userPath, "thumbs"),
		DBPath:      filepath.Join(userPath, "umbela.db"),
		ZipDepth: 	 3,
		APIPort:     41110,
		TorEnabled:  true,
		OneHopTor:   true,
		TorEd25519:  nil,
		ThumbWidth:  400,
		ThumbHeight: 0,
		WorkerCount: 4,
	}, nil
}
