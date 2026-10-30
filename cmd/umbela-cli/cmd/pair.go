package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/tor"

	"github.com/cretz/bine/torutil"
	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
)

var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Generate a QR code to pair your mobile app",
	Args:  cobra.ExactArgs(1),
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

	// 1. Cargar la configuración cifrada
	globalCfg, err := config.Read(userSecrets.VaultPath, userSecrets.KEK)
	if err != nil {
		return fmt.Errorf("error leyendo configuración: %w", err)
	}

	// 2. Inicializar TorController para obtener el dominio .onion actual
	torController, err := tor.NewController(&globalCfg.Server)
	if err != nil {
		return fmt.Errorf("error inicializando controlador de Tor: %w", err)
	}

	// Aseguramos que exista o recupere la llave Ed25519
	key, err := torController.CreateOrRecoverKey()
	if err != nil {
		return fmt.Errorf("error obteniendo clave Tor: %w", err)
	}

	pubKey := key.PublicKey()
	pkHex := base64.RawURLEncoding.EncodeToString(pubKey)

	// 4. Convertir el Salt del usuario a Hexadecimal
	saltHex := base64.RawURLEncoding.EncodeToString(userSecrets.Salt)

	onion:=torutil.OnionServiceIDFromPublicKey(pubKey)+".onion"

	// 5. Construir la URI para la app móvil
	// Usamos globalCfg.Server.APIPort para mantenerlo dinámico
	pairURI := fmt.Sprintf(
		"umbela://pair?onion=%s&port=%d&salt=%s&pk=%s",
		onion,
		80,
		saltHex,
		pkHex,
	)

	// 6. Mostrar información y dibujar el QR en la terminal
	fmt.Println("\nEscanea este código QR desde tu app Allium/Umbela:")
	fmt.Println("--------------------------------------------------")

	qrConfig := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    os.Stdout,
		BlackChar: qrterminal.BLACK_BLACK,
		WhiteChar: qrterminal.WHITE_WHITE,
		HalfBlocks: true,
		QuietZone: 1,
	}
	qrterminal.GenerateWithConfig(pairURI, qrConfig)

	fmt.Println("--------------------------------------------------")
	fmt.Printf("URI raw: %s\n\n", pairURI)

	return nil
}

func init() {
	rootCmd.AddCommand(pairCmd)
}