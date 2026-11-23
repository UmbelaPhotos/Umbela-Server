package vault

import (
	"context"
	"fmt"
	"sync"
	"umbela-server/internal/config"
	"umbela-server/internal/db"
	"umbela-server/internal/image"

	"github.com/uptrace/bun"
)

type Vault struct {
	path           string
	db             *bun.DB
	imageProcessor *image.Processor
	config         *config.VaultConfig

	// mu protects db and the DEK: requests hold it for reading (see Acquire),
	// Lock and Unlock take it for writing.
	mu sync.RWMutex

	// Face scanner lifecycle (see lock.go).
	scanMu     sync.Mutex
	scanParent context.Context
	scanCancel context.CancelFunc
	scanDone   chan struct{}
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

	v.db, err = db.InitDB(v.path, v.config.DEK)
	if err != nil {
		return err
	}
	fmt.Println("Connected to db")
	return nil
}

func (v *Vault) ShutDown() error {
	if v.imageProcessor != nil {
		v.imageProcessor.Shutdown()
		v.imageProcessor = nil
	}

	if v.db != nil {
		err := db.Close(v.db)
		v.db = nil
		if err != nil {
			return err
		}
	}

	return nil
}
