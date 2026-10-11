package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/config"
	"umbela-server/internal/db"
	"umbela-server/internal/image"
	"umbela-server/internal/tor"

	"github.com/uptrace/bun"
)

type Vault struct{
	Config 			*config.Config
	Path			string
	DB 				*bun.DB
	Iprocessor		*image.Processor
	TorController	*tor.Controller
	
	masterKey	[]byte
	salt		[]byte
}

func NewVault(path string, masterKey []byte, salt []byte) *Vault {
	return &Vault{
		masterKey: masterKey,
		salt: salt,
		Path: path,
	}
}

func (v *Vault) SetUp() error{
	// Make directories
	err:=v.CreateDirs()
	if err!=nil{
		return err
	}

	//  Config
	v.Config,err =config.DefaultConfig()
	if err!=nil{
		return err
	}

	// Tor address
	t,err:=tor.NewController(v.Config.APIPort)
	if err!=nil{
		return err
	}
	// Save ed25519 key on config
	_, err=t.CreateOrRecoverKey(v.Config)
	if err!=nil{
		return err
	}

	// Save for the first time
	v.SaveConfigFile()

	// Database
	v.DB, err = db.InitDB(v.Path, v.masterKey)
	if err != nil{
		return err
	}
	// TODO: Incorporar esto al bloqueo de la vault, defer.
	v.DB.Close()

	return nil
}

func (v *Vault) CreateDirs() error{	
	err:=os.MkdirAll(v.Path, 0700)
	if err!=nil{
		return fmt.Errorf("creating user dir: %w", err)
	}

	photosPath := filepath.Join(v.Path,"images")
	err=os.MkdirAll(photosPath, 0700)
	if err != nil{
		return fmt.Errorf("creating image dir: %w", err)
	}

	return nil
}	
