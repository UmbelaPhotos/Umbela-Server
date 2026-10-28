package cmd

import (
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/tor"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: 	"start",
	Short:  "Start your photos server",
	Args:	cobra.ExactArgs(1),
	RunE: 	RequireAuth(StartServer),
}

func StartServer(cmd *cobra.Command, args []string) error{
	ctx := cmd.Context()
	userSecrets, err := GetContextSecrets(ctx)
	if err != nil {
		return err
	}
	defer func() {
    	crypto.CleanBytes(userSecrets.KEK)
	}()

	globalCfg, err := config.Read(userSecrets.VaultPath, userSecrets.KEK)

	torController, err := tor.NewController(&globalCfg.Server)
	if err != nil{
		return err
	}

	return nil
}
