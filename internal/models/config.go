package models

import (
	"os"
	"path/filepath"
)

// Config agrupa toda la configuración en tiempo de ejecución del servidor Allium.
type Config struct {
	DataDir    string `json:"data_dir"`
	ThumbsDir  string `json:"thumbs_dir"`
	DBPath     string `json:"db_path"`

	APIPort    int  `json:"api_port"`
	TorEnabled bool `json:"tor_enabled"`

	ThumbWidth  int `json:"thumb_width"`
	ThumbHeight int `json:"thumb_height"`
	WorkerCount int `json:"worker_count"`
}

// DefaultConfig devuelve configuración con rutas en el home del usuario.
// Funciona igual ejecutes el binario desde donde sea.
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
		ThumbWidth:  400,
		ThumbHeight: 0,
		WorkerCount: 4,
	}
}
