package vault

import "umbela-server/internal/models"

// # Lee/Escribe el .salt y el .config.enc

type Vault struct{
	config 	models.Config
	salt 	[]byte
	path	string
}

func CreateVault(){
	
}