package tor

import (
	"context"
	"fmt"
	"os"
	"umbela-server/internal/config"

	"github.com/cretz/bine/tor"
	"github.com/cretz/bine/torutil/ed25519"
)

type Controller struct {
	dataDir      string
	APIPort      int
	onionAddress string
	onionID      string
	tor          *tor.Tor
	onion        *tor.OnionService
}

func NewController(APIPort int) (*Controller, error) {
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
		APIPort: APIPort,
	}, nil
}

func (c *Controller) CreateOrRecoverKey(cfg *config.Config) (ed25519.KeyPair, error) {
	var key ed25519.KeyPair

	if len(cfg.TorEd25519) != 0 {
		key = ed25519.FromCryptoPrivateKey(cfg.TorEd25519)
	} else {
		var err error
		key, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("error generando key: %w", err)
		}
		cfg.TorEd25519 = key.PrivateKey()
	}
	return key, nil
}

func (c *Controller) Start(ctx context.Context, cfg *config.Config) error {
	key, err := c.CreateOrRecoverKey(cfg)
	if err != nil {
		return err
	}

	t, err := tor.Start(ctx, &tor.StartConf{DataDir: c.dataDir})
	if err != nil {
		return fmt.Errorf("starting TOR engine %w", err)
	}

	// Connecting to the TOR network
	err = t.EnableNetwork(ctx, true)
	if err != nil {
		t.Close()
		return fmt.Errorf("enable network tor connection: %w", err)
	}

	onion, err := t.Listen(ctx, &tor.ListenConf{
		RemotePorts: []int{80},
		LocalPort:   c.APIPort,
		Key:         key,
	})

	if err != nil {
		t.Close()
		return fmt.Errorf("error creando hidden service: %w", err)
	}

	// save variables
	c.tor = t
	c.onion = onion
	c.onionID = onion.ID
	c.onionAddress = fmt.Sprintf("http://%s.onion", onion.ID)
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

func (c *Controller) OnionAddress() string {
	return c.onionAddress
}
