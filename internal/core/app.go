// Package core es el "cerebro" de Allium. Coordina todos los subsistemas:
// base de datos, procesamiento de imágenes, servidor HTTP y red Tor.
//
// La App es el punto de entrada único para toda la funcionalidad de negocio.
// Tanto el binario CLI (allium-node) como el GUI (allium-desktop) crean
// una instancia de App y llaman a sus métodos.
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

// App es el núcleo de la aplicación Allium.
// Contiene y coordina todos los subsistemas.
type App struct {
	cfg    models.Config
	db     *bun.DB
	server *api.Server
	log    *slog.Logger
	// TODO: añadir cuando se implementen los paquetes:
	processor *image.Processor
	torCtrl   *tor.Controller
	photoRepo *storage.PhotoRepository
}

// New crea una nueva instancia de App con la configuración dada.
// No arranca ningún servicio; llamar Start() para eso.
//
// Input:  cfg models.Config — configuración completa
// Output: *App, error si la configuración es inválida
func New(cfg models.Config) (*App, error) {
	// TODO: Implementar validación de cfg
	// 1. Verificar que cfg.DataDir no está vacío
	// 2. Verificar que cfg.WorkerCount > 0
	logger := slog.Default()
	return &App{
		cfg: cfg,
		log: logger,
	}, nil
}

// Start arranca todos los subsistemas en orden de dependencias:
//  1. Base de datos (siempre primero)
//  2. Procesador de imágenes
//  3. Servidor HTTP
//  4. Tor (si está habilitado en cfg)
//
// Si algún subsistema falla, hace rollback de los anteriores.
//
// Input:  ctx context.Context — cancelar esto apaga la app
// Output: error si algún subsistema no puede arrancar
func (a *App) Start(ctx context.Context) error {
	a.log.Info("arrancando Allium", "version", "0.1.0-dev")

	// [1] Base de datos
	database, err := db.InitDB(a.cfg.DBPath)
	if err != nil {
		return fmt.Errorf("fallo al iniciar BD: %w", err)
	}
	a.db = database
	a.log.Info("base de datos lista", "path", a.cfg.DBPath)

	// [2] Procesador de imágenes
	// TODO: cuando image.Processor esté implementado:
	processor, err := image.NewProcessor(a.cfg.ThumbsDir, a.cfg.ThumbWidth, a.cfg.ThumbHeight)
	if err != nil { return fmt.Errorf("fallo al iniciar procesador: %w", err) }
	a.processor = processor
	a.log.Info("procesador de imágenes pendiente de implementación")

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
