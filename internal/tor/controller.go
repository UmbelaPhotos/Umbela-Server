package tor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
	"umbela-server/internal/config"

	"github.com/briandowns/spinner"
	"github.com/cretz/bine/tor"
	"github.com/cretz/bine/torutil/ed25519"
)

type Controller struct {
	dataDir      string
	config       *config.ServerConfig
	onionAddress string
	onionID      string
	tor          *tor.Tor
	onion        *tor.OnionService
}

func NewController(cfg *config.ServerConfig) (*Controller, error) {
	tempDir, err := os.MkdirTemp("", "umbela-tor-*")
	if err != nil {
		return nil, fmt.Errorf("Error making the directory %w", err)
	}

	if err := os.Chmod(tempDir, 0700); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("changing permissions to temp dir: %w", err)
	}

	return &Controller{
		dataDir: tempDir,
		config:  cfg,
	}, nil
}

func (c *Controller) CreateOrRecoverKey() (ed25519.KeyPair, error) {
	var key ed25519.KeyPair

	if len(c.config.TorEd25519) != 0 {
		key = ed25519.PrivateKey(c.config.TorEd25519).KeyPair()
	} else {
		var err error
		key, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("error generando key: %w", err)
		}
		c.config.TorEd25519 = key.PrivateKey()
	}
	return key, nil
}

func (c *Controller) Start(ctx context.Context) error {
	key, err := c.CreateOrRecoverKey()
	if err != nil {
		return err
	}

	startConf := &tor.StartConf{
		DataDir:     c.dataDir,
		DebugWriter: io.Discard,
		NoHush: 	 true,
		ExtraArgs:   []string{"--quiet"},
	}
	if c.config.OneHopTor {
		startConf.NoAutoSocksPort = true
		startConf.ExtraArgs = append(startConf.ExtraArgs,
			"--SocksPort", "0",
			"--HiddenServiceSingleHopMode", "1",
			"--HiddenServiceNonAnonymousMode", "1",
		)
	}

	s := spinner.New(spinner.CharSets[43], 100*time.Millisecond)
	s.Color("magenta")
	s.Suffix = "Starting Tor Engine..."
	s.Start()
	t, err := tor.Start(ctx, startConf)

	if err != nil {
		return fmt.Errorf("starting TOR engine %w", err)
	}

	// Connecting to the TOR network
	s.Suffix = "Connecting to the Tor Network..."
	err = t.EnableNetwork(ctx, true)
	if err != nil {
		t.Close()
		return fmt.Errorf("enable network tor connection: %w", err)
	}

	s.Suffix = "Starting listening..."
	onion, err := t.Listen(ctx, &tor.ListenConf{
		RemotePorts:  []int{80},
		LocalPort:    c.config.APIPort,
		Key:          key,
		Version3:     true,
		NonAnonymous: c.config.OneHopTor,
	})
	if err != nil {
		t.Close()
		return fmt.Errorf("error creando hidden service: %w", err)
	}

	s.Suffix = "All done!"
	s.Stop()

	// save variables
	c.tor = t
	c.onion = onion
	c.onionID = onion.ID
	c.onionAddress = fmt.Sprintf("%s.onion", onion.ID)
	return nil
}

func (c *Controller) Stop() error {
	var errs []error

	if c.onion != nil {
		if err := c.onion.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing onion service: %w", err))
		}
	}

	if c.tor != nil {
		if err := c.tor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("shutting down tor process: %w", err))
		}
	}

	if c.dataDir != "" {
		if err := os.RemoveAll(c.dataDir); err != nil {
			errs = append(errs, fmt.Errorf("eliminating temp dir %s: %w", c.dataDir, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}

	return nil
}

// LocalListener devuelve el listener local en APIPort que bine abre en Start;
// Tor reenvia ahi el trafico del servicio onion.
func (c *Controller) LocalListener() net.Listener {
	return c.onion.LocalListener
}

func (c *Controller) OnionAddress() string {
	return c.onionAddress
}

const identifyContext = "umbela-identify-v1"

func (c *Controller) SignNonce(nonce []byte) ([]byte, error) {
	if len(c.config.TorEd25519) == 0 {
		return nil, errors.New("la clave privada Ed25519 de Tor no está inicializada")
	}

	privKey := ed25519.PrivateKey(c.config.TorEd25519)

	msg := append([]byte(identifyContext), nonce...)
	signature := ed25519.Sign(privKey, msg)

	return signature, nil
}
