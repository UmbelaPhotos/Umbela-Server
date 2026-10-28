package cmd

import (
	"fmt"
	"path/filepath"
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/tor"
	"umbela-server/internal/vault"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create your own profile to start Umbela",
	Args:  cobra.ExactArgs(0),
	RunE:  CreateProfile,
}

func CreateProfile(cmd *cobra.Command, args []string) error {
	// Authentication
	var username string
	fmt.Print("Insert username: ")
	fmt.Scanln(&username)

	passwordBytes, err := ReadPasswordClean("Create a password:")
	if err != nil {
		return fmt.Errorf("Couldn't read password")
	}

	confirmPasswordBytes, err := ReadPasswordClean("Confirm password: ")
	if err != nil {
		return fmt.Errorf("Couldn't read the confirmation")
	}

	if string(passwordBytes) != string(confirmPasswordBytes) {
		return fmt.Errorf("Passwords didn't match")
	}

	salt, err := crypto.RandomBytes(16)
	if err != nil {
		return err
	}

	KEK := crypto.DeriveKEK(passwordBytes, salt)

	hashedUsername := crypto.HashText(username, salt)
	_, baseDir, err := config.GetExecutableAndUmbelaPath()
	if err!=nil{
		return err
	}

	userPath := filepath.Join(baseDir, hashedUsername)

	globalCfg, err := config.DefaultConfig(userPath)

	userVault := vault.NewVault(userPath, &globalCfg.Vault)
	userVault.SetUp()

	torController, err := tor.NewController(&globalCfg.Server)
	if err!=nil{
		return err
	}
	_, err = torController.CreateOrRecoverKey()
	if err!=nil{
		return err
	}

	err = globalCfg.Save(KEK, salt)
	if err!=nil{
		return err
	}

	crypto.CleanBytes(passwordBytes, confirmPasswordBytes, KEK, salt)
	return nil
}

func init() {
	rootCmd.AddCommand(createCmd)
}
