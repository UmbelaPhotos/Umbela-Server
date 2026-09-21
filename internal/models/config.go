package models

import (
	"os"
	"path/filepath"
)

// Config contains all the runtime config of the Allium server.
type Config struct {
	DataDir    string `json:"data_dir"`
	ThumbsDir  string `json:"thumbs_dir"`
	DBPath     string `json:"db_path"`

	APIPort    int  `json:"api_port"`
	TorEnabled bool `json:"tor_enabled"` // TODO: Implemented to use local network when home 
	OneHopTor bool `json:"one_hop_enabled"`

	ThumbWidth  int `json:"thumb_width"`
	ThumbHeight int `json:"thumb_height"`
	WorkerCount int `json:"worker_count"`
}

// DefaultConfig returns the configuration with the user home paths.
func DefaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dataDir := filepath.Join(home, ".allium")
	return Config{
		DataDir:     dataDir,
		ThumbsDir:   filepath.Join(dataDir, "thumbs"),
		DBPath:      filepath.Join(dataDir, "allium.db"),
		APIPort:     41110,
		TorEnabled:  false,
		OneHopTor: 	 true,
		ThumbWidth:  400,
		ThumbHeight: 0,
		WorkerCount: 4,
	}
}
