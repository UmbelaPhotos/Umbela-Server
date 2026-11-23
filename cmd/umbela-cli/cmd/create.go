package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/tor"
	"umbela-server/internal/vault"
	"uuid"

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
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Insert username: ")
	text, _ := reader.ReadString('\n')
	username := strings.TrimSpace(text)
	fmt.Print("\033[1A\033[2K\r")

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

	salt, err := config.GetSalt()
	if err != nil {
		return err
	}

	KEK := crypto.DeriveKEK(username, passwordBytes, salt)

	// Check if there is already a user with that username and password combination
	_, err = vault.FindUserFolder(KEK)
	if err == nil {
		return vault.ErrUserAlreadyExists
	}

	folderName := uuid.NewV4().String()
	baseDir, err := config.UmbelaDir()
	if err != nil {
		return err
	}

	userPath := filepath.Join(baseDir, folderName)

	globalCfg, err := config.DefaultConfig(userPath)

	userVault := vault.NewVault(userPath, &globalCfg.Vault)
	if err = userVault.SetUp(); err != nil {
		userVault.ShutDown()
		return err
	}
	defer func() {
		if err := userVault.ShutDown(); err != nil {
			fmt.Printf("closing vault: %v\n", err)
		}
	}()

	torController, err := tor.NewController(&globalCfg.Server)
	if err != nil {
		return err
	}
	_, err = torController.CreateOrRecoverKey()
	if err != nil {
		return err
	}

	err = globalCfg.Save(KEK)
	if err != nil {
		return err
	}
	fmt.Println("User saved correctly")
	crypto.CleanBytes(passwordBytes, confirmPasswordBytes, KEK, salt)
	return nil
}

func init() {
	rootCmd.AddCommand(createCmd)
}
