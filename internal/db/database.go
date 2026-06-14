// Package db maneja la inicialización y configuración de la base de datos SQLite
// usando Bun ORM con el driver pure-Go modernc.org/sqlite.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"allium-server/internal/models"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

// InitDB abre (o crea si no existe) la base de datos SQLite en dbPath,
// activa el modo WAL para mejor rendimiento concurrente, y auto-migra
// los modelos del dominio.
//
// Input:  dbPath string — ruta al archivo .db (ej: "./data/allium.db")
// Output: *bun.DB listo para uso, o error si falla algo.
func InitDB(dbPath string) (*bun.DB, error) {
	// 1. Asegurar que la carpeta existe
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio: %w", err)
	}

	// 2. Abrir la conexión estándar de SQL
	sqldb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 3. Activar WAL mode para mejor rendimiento con múltiples readers
	if _, err := sqldb.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("error activando WAL: %w", err)
	}
	// Foreign Keys deben activarse por conexión en SQLite
	if _, err := sqldb.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("error activando foreign_keys: %w", err)
	}

	// 4. Envolver la conexión con Bun
	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Registrar modelos m2m antes de cualquier query
	db.RegisterModel((*models.AlbumPhoto)(nil))

	// 5. Auto-migrar todos los modelos del dominio
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("error en migraciones: %w", err)
	}

	return db, nil
}

// runMigrations crea las tablas del dominio si no existen.
// Agregua aquí cada nuevo modelo que necesite persistencia.
//
// Input:  *bun.DB ya inicializado
// Output: error si alguna CREATE TABLE falla
func runMigrations(db *bun.DB) error {
	ctx := context.Background()

	models := []interface{}{
		(*models.User)(nil),
		(*models.Session)(nil),
		(*models.Photo)(nil),
		(*models.Album)(nil),
		(*models.AlbumPhoto)(nil),
	}

	for _, model := range models {
		_, err := db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("migrando %T: %w", model, err)
		}
	}

	return nil
}