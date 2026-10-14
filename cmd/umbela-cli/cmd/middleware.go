package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/crypto"
	"umbela-server/internal/db"
	"umbela-server/internal/vault"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type contextKey string

const (
	CtxKeyMasterKey contextKey = "masterKey"
	CtxKeyVaultPath contextKey = "vaultPath"
	CtxKeySalt		contextKey = "salt"
)

type CommandRun func(cmd *cobra.Command, args []string) error

type Middleware func(CommandRun) CommandRun

func Chain(finalFunc CommandRun, middlewares ...Middleware) CommandRun {
	for i := len(middlewares) - 1; i >= 0; i-- {
		// First one in the list is the first one to check
		finalFunc = middlewares[i](finalFunc)
	}
	return finalFunc
}

func RequireAuth(next CommandRun) CommandRun {
	return func(cmd *cobra.Command, args []string) error {
		username := args[0]
		passwordBytes, err := ReadPasswordClean("Enter your password: ") 

		if err != nil {
			return err
		}

		userFolder, userSalt, err := vault.FindUserFolder(username)
		if errors.Is(err, vault.ErrUserNotFound) {
			return ErrUserOrPasswordIncorrect
		} else if err != nil {
			return fmt.Errorf("system error: %w", err)
		}

		masterKey := crypto.DeriveMasterKey(passwordBytes, userSalt)
		dbPath := filepath.Join(userFolder, "database.db")

		// Check password
		err = db.TestUnlock(dbPath, masterKey)
		if errors.Is(err, db.ErrInvalidDatabasePassword) {
			return ErrUserOrPasswordIncorrect
		} else if err!=nil{
			return err
		}
		

		ctx := cmd.Context()
		ctx = context.WithValue(ctx, CtxKeyMasterKey, masterKey)
		ctx = context.WithValue(ctx, CtxKeyVaultPath, userFolder)
		ctx = context.WithValue(ctx, CtxKeySalt, userSalt)

		cmd.SetContext(ctx)

		return next(cmd, args)
	}
}

func RequireUsername(username string) (string, error) {
	// Check for the config file
	userVault,_, err := vault.FindUserFolder(username)

	if errors.Is(err, vault.ErrUserNotFound) {
		return "", err
	}

	return userVault, nil
}

func ReadPasswordClean(textPrompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("input is not a terminal TTY")
	}

	fmt.Print(textPrompt)

	passwordBytes, err := term.ReadPassword(fd)
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	fmt.Println()

	return passwordBytes, nil
}

func RequireServerNotRunning(next CommandRun) CommandRun {
	return func(cmd *cobra.Command, args []string) error {
		isRunning := false // TODO: checar si el puerto gRPC ya está en uso

		if isRunning {
			return fmt.Errorf("el servidor ya está corriendo")
		}

		return next(cmd, args)
	}
}

type ContextSecrets struct {
	MasterKey []byte
	VaultPath string
	Salt      []byte
}

func GetContextSecrets(ctx context.Context) (ContextSecrets, error) {
	masterKey, ok := ctx.Value(CtxKeyMasterKey).([]byte)
	if !ok || len(masterKey) == 0 {
		return ContextSecrets{}, fmt.Errorf("master key no encontrada en el contexto")
	}

	vaultPath, ok := ctx.Value(CtxKeyVaultPath).(string)
	if !ok || vaultPath == "" {
		return ContextSecrets{}, fmt.Errorf("vault path no encontrado en el contexto")
	}

	salt, ok := ctx.Value(CtxKeySalt).([]byte)
	if !ok || len(salt) == 0 {
		return ContextSecrets{}, fmt.Errorf("salt no encontrado en el contexto")
	}

	return ContextSecrets{
		MasterKey: masterKey,
		VaultPath: vaultPath,
		Salt:      salt,
	}, nil
}