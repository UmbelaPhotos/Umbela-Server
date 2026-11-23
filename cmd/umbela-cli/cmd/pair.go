package cmd

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/tor"

	"github.com/cretz/bine/torutil"
	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	altScreenOn  = "\033[?1049h"
	altScreenOff = "\033[?1049l"
	clearScreen  = "\033[2J\033[3J\033[H"
	keyCtrlC     = 0x03
	keyIgnore    = 0xFF // secuencia de escape (flechas, Esc, F1...) o varias teclas a la vez
)

var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Generate a QR code to pair your mobile app",
	Args:  cobra.ExactArgs(0),
	RunE:  RequireAuth(Pair),
}

func Pair(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("reading configuration: %w", err)
	}

	torController, err := tor.NewController(&globalCfg.Server)
	if err != nil {
		return fmt.Errorf("new tor controller: %w", err)
	}

	key, err := torController.CreateOrRecoverKey()
	if err != nil {
		return fmt.Errorf("retrieving tor key: %w", err)
	}

	pubKey := key.PublicKey()
	pkHex := base64.RawURLEncoding.EncodeToString(pubKey)
	saltHex := base64.RawURLEncoding.EncodeToString(userSecrets.Salt)
	onion := torutil.OnionServiceIDFromPublicKey(pubKey) + ".onion"

	// Par x25519 del cliente. Tor lo quiere en base32 mayúsculas, sin padding.
	b32 := base32.StdEncoding.WithPadding(base32.NoPadding)
	clientKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating client auth key: %w", err)
	}
	privBytes := clientKey.Bytes()
	defer crypto.CleanBytes(privBytes)
	clientPriv := b32.EncodeToString(privBytes)
	clientPub := b32.EncodeToString(clientKey.PublicKey().Bytes())

	pairURI := fmt.Sprintf(
		"umbela://pair?onion=%s&port=%d&salt=%s&pk=%s&auth=%s",
		onion, 80, saltHex, pkHex, clientPriv,
	)

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("pair needs an interactive terminal")
	}

	// Pantalla alterna: el QR (con la clave privada) nunca entra al scrollback.
	fmt.Print(altScreenOn)

	save := false
	for {
		fmt.Print(clearScreen)
		fmt.Println("Scan this code in your Umbela App:")
		fmt.Println("--------------------------------------------------")
		qrterminal.GenerateWithConfig(pairURI, qrterminal.Config{
			Level:      qrterminal.M,
			Writer:     os.Stdout,
			BlackChar:  qrterminal.BLACK_BLACK,
			WhiteChar:  qrterminal.WHITE_WHITE,
			HalfBlocks: true,
			QuietZone:  1,
		})
		fmt.Println("--------------------------------------------------")
		fmt.Printf("URI raw: %s\n\n", pairURI)
		fmt.Println("Press ENTER (or any key) once the app has scanned it to authorize this device.")
		fmt.Println("Press q or Ctrl+C to cancel; nothing will be saved.")

		k, err := waitKey()
		if err != nil || k == keyCtrlC || k == 'q' || k == 'Q' {
			break // cancelado: no se guarda
		}
		if k == keyIgnore {
			continue // flechas, Esc, etc.: no cuentan
		}
		save = true
		break
	}

	// Borrar QR y URI de la pantalla y del scrollback.
	fmt.Print(clearScreen, altScreenOff)

	if !save {
		fmt.Println("Pairing cancelled. The device was NOT authorized.")
		return nil
	}

	globalCfg.Server.AuthorizedClients = append(globalCfg.Server.AuthorizedClients, clientPub)
	if err := globalCfg.Save(userSecrets.KEK); err != nil {
		return fmt.Errorf("saving authorized client: %w", err)
	}
	fmt.Println("Device authorized. Restart the server for the change to take effect.")
	return nil
}

// waitKey lee una sola tecla en modo raw (en raw, Ctrl+C llega como 0x03).
func waitKey() (byte, error) {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer term.Restore(fd, old)

	// Una flecha llega como "\x1b[A" (3 bytes) en una sola lectura: se lee todo
	// junto para no tomar solo el primer byte como una tecla normal.
	buf := make([]byte, 32)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return 0, err
	}
	if buf[0] == 0x1b || n > 1 {
		return keyIgnore, nil
	}
	return buf[0], nil
}

func init() {
	rootCmd.AddCommand(pairCmd)
}