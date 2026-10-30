package cmd

import (
	"fmt"
	"umbela-server/internal/api/server"
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/tor"
	"umbela-server/internal/vault"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start your photos server securely over Tor",
	Args:  cobra.ExactArgs(1),
	RunE:  RequireAuth(StartServer), 
}

func StartServer(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	
	userSecrets, err := GetContextSecrets(ctx)
	if err != nil {
		return err
	}
	defer func() {
		crypto.CleanBytes(userSecrets.KEK)
	}()

	globalCfg, err := config.Read(userSecrets.VaultPath, userSecrets.KEK)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	torController, err := tor.NewController(&globalCfg.Server)
	if err != nil {
		return fmt.Errorf("creating new tor controller: %w", err)
	}

	userVault := vault.NewVault(userSecrets.VaultPath, &globalCfg.Vault)
	if err = userVault.SetUp(); err != nil{
		return err
	}
	return server.Run(ctx, &globalCfg.Server, userVault, torController)
}

func init() {
	rootCmd.AddCommand(startCmd)
}