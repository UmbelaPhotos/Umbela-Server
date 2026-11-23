package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"umbela-server/internal/api/server"
	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/daemon"
	"umbela-server/internal/tor"
	"umbela-server/internal/vault"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

// readyMarker is the line server.Run prints once the onion service is up.
const readyMarker = "Tor service listening on:"

// daemonMode is only true when the program launches itself in the background.
var daemonMode bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start your photos server in the background over Tor",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		if daemonMode {
			cmd.SilenceUsage = true // the parent prints our error, no usage text
			return runDaemon(cmd)
		}
		return RequireAuth(StartServer)(cmd, args)
	},
}

// StartServer launches the server in the background and waits until the onion service is running.
func StartServer(cmd *cobra.Command, args []string) error {
	secrets, err := GetContextSecrets(cmd.Context())
	if err != nil {
		return err
	}
	defer crypto.CleanBytes(secrets.KEK)

	if pid, ok := daemon.Running(secrets.VaultPath); ok {
		return fmt.Errorf("server is already running (pid %d)", pid)
	}

	child, err := daemon.Spawn(secrets.KEK, "start", "--daemon")
	if err != nil {
		return err
	}

	// Ctrl+C while starting aborts the launch instead of leaving a half started server.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := spinner.New(spinner.CharSets[43], 100*time.Millisecond)
	s.Color("magenta")
	s.Suffix = " Starting server..."
	s.Start()
	defer s.Stop()

	var output []string
	for {
		select {
		case <-ctx.Done():
			s.Suffix = " Cancelling..."
			child.Stop(secrets.VaultPath)
			s.Stop()
			return fmt.Errorf("start cancelled")
		case line, open := <-child.Lines:
			if !open { // the child died before the onion service was up
				s.Stop()
				return fmt.Errorf("server stopped while starting:\n%s", strings.Join(output, "\n"))
			}
			output = append(output, line)
			switch {
			case strings.Contains(line, readyMarker):
				s.Stop()
				fmt.Println("Onion service running in the background")
				return nil
			case strings.Contains(line, "Connected to db"):
				s.Suffix = " Connecting to the Tor Network..."
			}
		}
	}
}

// runDaemon is the background process: it reads the KEK from stdin and runs the server until it is stopped.
func runDaemon(cmd *cobra.Command) error {
	daemon.Init()

	kek, err := io.ReadAll(os.Stdin)
	if err != nil || len(kek) == 0 {
		return fmt.Errorf("reading key from parent: %v", err)
	}
	defer crypto.CleanBytes(kek)

	vaultPath, err := vault.FindUserFolder(kek)
	if err != nil {
		return err
	}

	if err := daemon.Acquire(vaultPath); err != nil {
		return err
	}
	defer daemon.Release(vaultPath)

	// SIGTERM cancels ctx too, so a stop during the Tor start also cleans up.
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// The stop file is how "stop" reaches us on Windows.
	go daemon.WatchStop(vaultPath, ctx.Done(), cancel)

	globalCfg, err := config.Read(vaultPath, kek)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	torController, err := tor.NewController(&globalCfg.Server)
	if err != nil {
		return fmt.Errorf("creating new tor controller: %w", err)
	}

	userVault := vault.NewVault(vaultPath, &globalCfg.Vault)
	if err = userVault.SetUp(); err != nil {
		userVault.ShutDown()
		return err
	}
	defer func() {
		if err := userVault.ShutDown(); err != nil {
			fmt.Printf("closing vault: %v\n", err)
		}
	}()

	return server.Run(ctx, &globalCfg.Server, userVault, torController)
}

func init() {
	startCmd.Flags().BoolVar(&daemonMode, "daemon", false, "internal: run as the background process")
	startCmd.Flags().MarkHidden("daemon")
	rootCmd.AddCommand(startCmd)
}
