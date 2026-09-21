// The package db manages the creation and configuration of the SQLite database
// it uses Bun ORM with the driver go-sqlite3.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"allium-server/internal/models"

	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// InitDB opens (or creates) the SQLite database in dbPath,
// activates the WAL mode for better concurrency, and automigrates
// the models of the domain
//
// Input:  dbPath string — path to the .db file (example: "./data/allium.db")
// Output: *bun.DB ready to use, or error.
func InitDB(dbPath string, username string, password string) (*bun.DB, error) {
	// Make sure the dir exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	values := url.Values{}
	values.Add("_journal_mode", "WAL")
	values.Add("_foreign_keys", "ON")
	values.Add("_busy_timeout", "5000")

	dsn := fmt.Sprintf("%s?%s", dbPath, values.Encode())

	// 2. open standard sql connection
	sqldb, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	var success bool
	defer func() {
		if !success {
			sqldb.Close()
		}
	}()

	sqldb.SetMaxOpenConns(1)

	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("verifying connection: %w", err)
	}

	// 4. Connection wit bun
	db := bun.NewDB(sqldb, sqlitedialect.New())

	success = true

	// Register m2m models
	db.RegisterModel((*models.AlbumPhoto)(nil))

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("error en migraciones: %w", err)
	}

	return db, nil
}

// runMigrations creates the tables.
// Here goes the new models that require persistance
//
// Input:  *bun.DB
// Output: error if there is a error creating the table
func runMigrations(db *bun.DB) error {
	ctx := context.Background()

	models := []interface{}{
		(*models.Face)(nil),
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
