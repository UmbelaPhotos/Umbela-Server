package cmd

import (
	"fmt"
	"path/filepath"
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/vault"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:	"create",
	Short:	"Create your own profile to start Umbela",
	Args:	cobra.ExactArgs(0),
	RunE:	CreateProfile,
}

func CreateProfile(cmd *cobra.Command, args []string) error{
	// Authentication
	var username string
	fmt.Print("Insert username: ")
	fmt.Scanln(&username)

	
	fmt.Print("Create a password: ")
	passwordBytes, err := ReadPasswordClean()
	if err!=nil{
		return fmt.Errorf("Couldn't read password")
	}

	fmt.Print("Confirm password: ")
	confirmPasswordBytes, err := ReadPasswordClean()
	if err!=nil{
		return fmt.Errorf("Couldn't read the confirmation")
	}

	if string(passwordBytes) != string(confirmPasswordBytes){
		return fmt.Errorf("Passwords didn't match")
	}
	
	salt, err:=crypto.GenerateSalt()
	if err!=nil{
		return err
	}
	
	masterKey := crypto.DeriveMasterKey(passwordBytes, salt)
	
	hashedUsername:=crypto.HashText(username, salt)
	_,baseDir,err:=config.GetExecutableAndUmbelaPath()
	userPath := filepath.Join(baseDir,hashedUsername)

	userVault:=vault.NewVault(userPath, masterKey, salt)

	userVault.SetUp()

	crypto.CleanBytes(passwordBytes, confirmPasswordBytes)
	return nil
}