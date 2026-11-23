// The package db manages the creation and configuration of the SQLite database
// it uses Bun ORM with the driver go-sqlite3.
package db

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"umbela-server/internal/models"

	_ "github.com/mutecomm/go-sqlcipher/v4"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// InitDB opens (or creates) the SQLite database in dbPath,
// activates the WAL mode for better concurrency, and automigrates
// the models of the domain
//
// Input:  dbPath string — path to the .db file (example: "./data/umbela.db")
// Output: *bun.DB ready to use, or error.
func InitDB(baseDir string, DEK []byte) (*bun.DB, error) {
	// Make sure the dir exists
	dbPath := filepath.Join(baseDir, "database.db")
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	hexKey := hex.EncodeToString(DEK)

	values := url.Values{}
	values.Add("_journal_mode", "WAL")
	values.Add("_foreign_keys", "ON")
	values.Add("_busy_timeout", "5000")

	// SQLCipher
	values.Add("_pragma_key", fmt.Sprintf("x'%s'", hexKey))
	values.Add("_pragma_cipher_page_size", "4096")

	dsn := fmt.Sprintf("%s?%s", dbPath, values.Encode())

	// 2. open standard sql connection
	sqldb, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
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

	// 4. Connection with bun
	db := bun.NewDB(sqldb, sqlitedialect.New())

	
	// Register m2m models
	db.RegisterModel((*models.AlbumPhoto)(nil))
	db.RegisterModel((*models.PhotoFace)(nil))
	
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("error en migraciones: %w", err)
	}
	
	success = true

	return db, nil
}

// Close flushes the WAL into the main database file and closes the connection.
// After a clean close SQLite removes the -wal and -shm files, so nothing is left
// next to the encrypted database. It is safe to call with a nil db.
//
// The caller must make sure no query is still running; Close does not wait for them.
func Close(db *bun.DB) error {
	if db == nil {
		return nil
	}

	_, ckptErr := db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")
	closeErr := db.Close()

	if ckptErr != nil {
		ckptErr = fmt.Errorf("checkpointing wal: %w", ckptErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("closing database: %w", closeErr)
	}
	return errors.Join(ckptErr, closeErr)
}

// runMigrations creates the tables.
// Here goes the new models that require persistance
//
// Input:  *bun.DB
// Output: error if there is a error creating the table
func runMigrations(db *bun.DB) error {
	ctx := context.Background()

	// A cover is optional (NULL) and has to be a photo that exists. When the
	// photo is deleted the cover is cleared by the database itself.
	const coverFK = `("cover_photo_id") REFERENCES "photos" ("id") ON DELETE SET NULL`

	tables := []struct {
		model any
		fks   []string
	}{
		// photos goes first: faces and albums point to it.
		{(*models.Photo)(nil), nil},
		{(*models.Face)(nil), []string{coverFK}},
		{(*models.Album)(nil), []string{coverFK}},
		{(*models.AlbumPhoto)(nil), nil},
		{(*models.PhotoFace)(nil), nil},
	}

	for _, t := range tables {
		q := db.NewCreateTable().Model(t.model).IfNotExists()
		for _, fk := range t.fks {
			q = q.ForeignKey(fk)
		}
		if _, err := q.Exec(ctx); err != nil {
			return fmt.Errorf("migrando %T: %w", t.model, err)
		}
	}

	// The face scanner looks for photos that were not scanned yet.
	if _, err := db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_photos_faces_pending ON photos (faces_scanned_at)`,
	); err != nil {
		return fmt.Errorf("creating face scan index: %w", err)
	}

	// The timeline pages by (captured_at, id) and the month buckets scan
	// captured_at; this index serves both.
	if _, err := db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_photos_captured_at_id ON photos (captured_at DESC, id DESC)`,
	); err != nil {
		return fmt.Errorf("creando índice del timeline: %w", err)
	}

	return nil
}
