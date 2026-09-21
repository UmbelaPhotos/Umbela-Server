// Package core is the "brain" of Allium. It controls the following packages:
// databases, image processing, http server and TOR network.
//
// The app is the entrance for all the funcionality.
// allium-node and allium-desktop create an instance of App and call its methods
package core

import (
	"context"
	"fmt"
	"log/slog"

	"allium-server/internal/api"
	"allium-server/internal/db"
	"allium-server/internal/models"
	"allium-server/internal/image"
	"allium-server/internal/storage"
	"allium-server/internal/tor"


	"github.com/uptrace/bun"
)

// App is the nuclei of Allium
// Contains and coordinates all its methods
type App struct {
	cfg    models.Config
	db     *bun.DB
	server *api.Server
	log    *slog.Logger
	processor *image.Processor
	torCtrl   *tor.Controller
	photoRepo *storage.PhotoRepository
}

// New creates a new instance with the configuration given
// Doesnt call any method, use Start() for that
//
// Input:  cfg models.Config — complete configuration
// Output: *App, error 
func New(cfg models.Config) (*App, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("invalid config: no value in DataDir")
	}
	if cfg.WorkerCount<=0{
		return nil, fmt.Errorf("invalid config: worker value less than 1")
	}

	logger := slog.Default()
	return &App{
		cfg: cfg,
		log: logger,
	}, nil
}

// Start starts all the subsistems in the following order:
//  1. Database
//  2. image processing
//  3. HTTP server # TODO: Remove
//  4. Tor (if enabled)
//
// If any subsistem fails, makes a rollback to the last one.
//
// Input:  ctx context.Context — cancel this shutsdown the app
// Output: error if a subsistem fails
func (a *App) Start(ctx context.Context) error {
	a.log.Info("Starting Allium", "version", "0.1.0-dev")

	// Database
	database, err := db.InitDB(a.cfg.DBPath)
	if err != nil {
		return fmt.Errorf("starting db: %w", err)
	}
	a.db = database
	a.log.Info("database ready", "path", a.cfg.DBPath)

	// image processing
	processor, err := image.NewProcessor(a.cfg.ThumbsDir, a.cfg.ThumbWidth, a.cfg.ThumbHeight)
	if err != nil { return fmt.Errorf("initializing image processor: %w", err) }
	defer processor.Shutdown()
	a.processor = processor
	a.log.Info("image processor ready")

	// [3] Servidor HTTP
	photoRepo := storage.NewPhotoRepository(a.db)
	albumRepo := storage.NewAlbumRepository(a.db)
	a.server = api.NewServer(a.cfg.APIPort, a.cfg.DataDir, a.db, photoRepo, albumRepo)
	go func() {
		if err := a.server.Start(); err != nil {
			a.log.Error("servidor HTTP se detuvo", "error", err)
		}
	}()
	a.log.Info("servidor HTTP arrancado", "port", a.cfg.APIPort)

	// [4] Tor (opcional)
	if a.cfg.TorEnabled {
		torCtrl, err := tor.NewController(a.cfg.DataDir, a.cfg.APIPort)
		if err != nil { return fmt.Errorf("fallo al crear controlador Tor: %w", err) }
		if err := torCtrl.Start(ctx); err != nil { return fmt.Errorf("fallo al arrancar Tor: %w", err) }
		a.torCtrl = torCtrl
		a.log.Info("Tor listo", "onion", torCtrl.OnionAddress())
	}

	a.log.Info("Allium listo 🧅")
	return nil
}

// Stop apaga todos los subsistemas de forma limpia en orden inverso.
// Llamar con defer desde main después de Start exitoso.
func (a *App) Stop() {
	a.log.Info("apagando Allium...")

	a.torCtrl.Stop()
	a.processor.Shutdown()
	a.db.Close()

	if a.db != nil {
		_ = a.db.Close()
	}
	a.log.Info("Allium apagado correctamente")
}

// Startup es el hook que Wails llama cuando la ventana está lista.
// Arranca todos los subsistemas con el contexto de Wails.
func (a *App) Startup(ctx context.Context) {
	if err := a.Start(ctx); err != nil {
		a.log.Error("error al arrancar", "error", err)
	}
}

// Shutdown es el hook que Wails llama cuando el usuario cierra la ventana.
func (a *App) Shutdown(_ context.Context) {
	a.Stop()
}

// DB expone la conexión a la BD para los repositorios.
// Usar solo en inicialización; los handlers no deben acceder a esto directamente.
func (a *App) DB() *bun.DB {
	return a.db
}
