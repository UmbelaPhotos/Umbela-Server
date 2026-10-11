package config

import (
	"path/filepath"
)

type Config struct {
	DataDir   string `json:"data_dir"`
	ThumbsDir string `json:"thumbs_dir"`
	DBPath    string `json:"db_path"`

	APIPort    int    `json:"api_port"`
	TorEnabled bool   `json:"tor_enabled"`
	OneHopTor  bool   `json:"one_hop_enabled"`
	TorEd25519 []byte `json:"tor_ed25519,omitempty"`

	ThumbWidth  int `json:"thumb_width"`
	ThumbHeight int `json:"thumb_height"`
	WorkerCount int `json:"worker_count"`
}

// DefaultConfig returns the configuration with the user home paths.
func DefaultConfig() (*Config, error) {
	_, umbelaDirectory, err := GetExecutableAndUmbelaPath()

	if err != nil {
		return nil, err
	}

	return &Config{
		DataDir:     umbelaDirectory,
		ThumbsDir:   filepath.Join(umbelaDirectory, "thumbs"),
		DBPath:      filepath.Join(umbelaDirectory, "umbela.db"),
		APIPort:     41110,
		TorEnabled:  true,
		OneHopTor:   true,
		TorEd25519:  nil,
		ThumbWidth:  400,
		ThumbHeight: 0,
		WorkerCount: 4,
	}, nil
}
