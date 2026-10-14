package vault

import (
	"umbela-server/internal/db"
)


func (v *Vault) Open(masterKey []byte) error {
	err := v.Config.ReadConfigFile(masterKey)
	if err != nil{
		return err
	}

	v.DB, err = db.InitDB(v.Path, v.masterKey)

	// Image pipeline 

	return nil
}