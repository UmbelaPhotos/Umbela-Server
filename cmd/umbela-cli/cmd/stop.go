package cmd

import (
	"fmt"
	"time"
	"umbela-server/internal/crypto"
	"umbela-server/internal/daemon"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop your photos server running in the background",
	Args:  cobra.ExactArgs(0),
	RunE:  RequireAuth(StopServer),
}

// StopServer asks the background server of the user to shut down and waits until it is gone.
func StopServer(cmd *cobra.Command, args []string) error {
	secrets, err := GetContextSecrets(cmd.Context())
	if err != nil {
		return err
	}
	defer crypto.CleanBytes(secrets.KEK)

	pid, ok := daemon.Running(secrets.VaultPath)
	if !ok {
		return fmt.Errorf("server is not running")
	}

	if err := daemon.Terminate(secrets.VaultPath, pid); err != nil {
		return fmt.Errorf("stopping server: %w", err)
	}

	// The server closes the database and Tor before it exits, so give it time.
	if !daemon.WaitStopped(secrets.VaultPath, 20*time.Second) {
		return fmt.Errorf("server did not stop after 20 seconds (pid %d)", pid)
	}

	fmt.Println("Server stopped")
	return nil
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
