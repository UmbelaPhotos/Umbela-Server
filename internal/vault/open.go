package vault

import (
	"umbela-server/internal/db"
)


func (v *Vault) Open() error {
	err := v.ReadConfigFile()
	if err != nil{
		return err
	}

	v.DB, err = db.InitDB(v.Path, v.masterKey)

	// Image pipeline 

	return nil
}