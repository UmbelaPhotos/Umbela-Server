// Package tor gestiona el ciclo de vida del daemon de Tor integrado.
// Utiliza github.com/cretz/bine para arrancar y controlar Tor programáticamente,
// sin necesidad de que el usuario instale o configure Tor manualmente.
//
// Flujo típico:
//  1. NewController(dataDir)
//  2. ctrl.Start(ctx)  → arranca Tor y crea el Hidden Service
//  3. ctrl.OnionAddress() → dirección .onion lista para compartir
//  4. ctrl.Stop()  → apagado limpio al cerrar la app
package tor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cretz/bine/tor"
	"github.com/cretz/bine/torutil/ed25519"
)

// Controller encapsula el proceso Tor embebido y el Hidden Service.
type Controller struct {
	dataDir      string // Directorio donde Tor guardará sus keys y estado
	apiPort      int    // Puerto local del servidor HTTP que se expondrá
	onionAddress string // Dirección .onion asignada (poblada después de Start)
	onionID 	 string
	tor          *tor.Tor
	onion        *tor.OnionService
}

// NewController crea un Controller pero NO arranca Tor todavía.
//
// Input:  dataDir string — carpeta para persistir las keys del Hidden Service
//
//	apiPort int    — puerto del servidor HTTP local a tunelizar
//
// Output: *Controller, error
func NewController(dataDir string, apiPort int) (*Controller, error) {
	// TODO: Implementar
	// 1. Validar que dataDir existe o crearlo con os.MkdirAll
	// 2. Devolver &Controller{dataDir, apiPort, ""}
	err := os.MkdirAll(dataDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("Error making the directory %w", err)
	}

	return &Controller{dataDir, apiPort, "", "", nil, nil}, nil
}

// Start arranca el proceso Tor embebido y espera a que esté listo.
// Puede tardar 30-60 segundos la primera vez que Tor bootstrappea.
// El ctx permite cancelar la espera (ej: usuario cierra la app).
//
// Input:  ctx context.Context — cancelable para timeouts
// Output: error si Tor no puede arrancar o bootstrappear
func (c *Controller) Start(ctx context.Context) error {
	// TODO: Implementar con bine:
	// 1. t, err := tor.Start(ctx, &tor.StartConf{DataDir: c.dataDir})
	// 2. Esperar bootstrap: t.EnableNetwork(ctx, true)
	// 3. Crear Hidden Service: t.Listen(ctx, &tor.ListenConf{RemotePorts: []int{80}, LocalPort: c.apiPort})
	// 4. Guardar la dirección .onion en c.onionAddress

	// Empezar el tor engine
	t, err := tor.Start(ctx, &tor.StartConf{DataDir: c.dataDir})
	if err != nil {
		return fmt.Errorf("Error starting TOR engine %w", err)
	}

	// Connecting to the TOR network
	err = t.EnableNetwork(ctx, true)
	if err != nil {
		t.Close()
		return fmt.Errorf("Couldnt connect to the TOR network: %w", err)
	}

	// Creating a hidden service
	keyPath := filepath.Join(c.dataDir, "onion_key")

	var key ed25519.KeyPair

	keyBytes, err := os.ReadFile(keyPath)
	if err == nil {
		// Ya existe, reutilizarla
		fmt.Println("Ya existe")
		key = ed25519.FromCryptoPrivateKey(keyBytes)
	} else {
		// No existe, generar y guardar
		fmt.Println("No existia")
		key, err = ed25519.GenerateKey(nil)
		if err != nil {
			t.Close()
			return fmt.Errorf("error generando key: %w", err)
		}
		os.WriteFile(keyPath, key.PrivateKey(), 0600)
	}

	onion, err := t.Listen(ctx, &tor.ListenConf{
		RemotePorts: []int{80},
		LocalPort:   c.apiPort,
		Key:         key,
	})

	if err != nil {
		t.Close()
		return fmt.Errorf("error creando hidden service: %w", err)
	}

	// Test remove after
	fmt.Printf("Your onion id is: http://%v.onion \n", onion.ID)

	// save variables
	c.tor = t
	c.onion = onion
	c.onionID = onion.ID
	c.onionAddress = fmt.Sprintf("http://%s.onion", onion.ID)
	return nil
}

// Stop detiene el proceso Tor de forma limpia.
// Llamar con defer después de Start exitoso.
func (c *Controller) Stop() {
	// TODO: Implementar
	// 1. Llamar t.Close() del paquete bine
	if c.onion != nil {
		c.onion.Close()
	}
	if c.tor != nil {
		c.tor.Close()
	}
}

// OnionAddress devuelve la dirección .onion del Hidden Service una vez que
// Start() ha completado exitosamente. Devuelve "" antes de eso.
//
// Output: string "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion"
func (c *Controller) OnionAddress() string {
		return c.onionAddress
}
