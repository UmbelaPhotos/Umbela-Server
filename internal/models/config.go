package models

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config contains all the runtime config of the umbela server.
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

func GetExecutableAndUmbelaPath()(string, string, error){
	exePath, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("No executable dir: %w", err)
	}
	
	exeDir:=filepath.Dir(exePath)
	umbelaDirectory := filepath.Join(exeDir, ".umbela")

	return exeDir, umbelaDirectory, nil 
}

// DefaultConfig returns the configuration with the user home paths.
func DefaultConfig() (*Config, error) {
	_,umbelaDirectory, err :=GetExecutableAndUmbelaPath()

	if err!=nil{
		return nil, err
	}

	return &Config{
		DataDir:     umbelaDirectory,
		ThumbsDir:   filepath.Join(umbelaDirectory, "thumbs"),
		DBPath:      filepath.Join(umbelaDirectory, "umbela.db"),
		APIPort:     41110,
		TorEnabled:  false,
		OneHopTor: 	 true,
		ThumbWidth:  400,
		ThumbHeight: 0,
		WorkerCount: 4,
	}, nil
}

