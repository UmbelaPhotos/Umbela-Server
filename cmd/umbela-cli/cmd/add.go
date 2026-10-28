package cmd

import (
	"fmt"
	"path/filepath"
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/vault"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add photos to your personal library",
	Args:  cobra.ExactArgs(2),
	RunE:  RequireAuth(AddImages),
}

func AddImages(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	userSecrets, err := GetContextSecrets(ctx)
	if err != nil {
		return err
	}
	defer func() {
    	crypto.CleanBytes(userSecrets.KEK)
	}()

	addImagesPath, err := filepath.Abs(args[1])
	if err != nil {
		return fmt.Errorf("converting to absolute path: %w\n", err)
	}
	fmt.Printf("processing images in path: %v\n", addImagesPath)

	globalCfg, err := config.Read(userSecrets.VaultPath, userSecrets.KEK)

	userVault := vault.NewVault(userSecrets.VaultPath, &globalCfg.Vault)
	
	err = userVault.AddPhotos(ctx, addImagesPath)
	if err != nil{
		return nil
	}

	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)
}
