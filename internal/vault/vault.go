package vault

import (
	"fmt"
	"umbela-server/internal/config"
	"umbela-server/internal/db"
	"umbela-server/internal/image"
	"umbela-server/internal/tor"

	"github.com/uptrace/bun"
)

type Vault struct {
	path           string
	db             *bun.DB
	imageProcessor *image.Processor
	torController  *tor.Controller

	dek    []byte
	config *config.VaultConfig
}

func NewVault(path string, cfg *config.VaultConfig) *Vault {
	return &Vault{
		path:   path,
		config: cfg,
	}
}

func (v *Vault) SetUp() error {
	var err error
	v.imageProcessor, err = image.NewProcessor(v.config.ThumbsDir, 320, 0)
	if err != nil {
		return err
	}
	fmt.Println("Initialized Govips")

	v.db, err = db.InitDB(v.path, v.dek)
	if err != nil {
		return err
	}
	fmt.Println("Connected to db")
	return nil
}
