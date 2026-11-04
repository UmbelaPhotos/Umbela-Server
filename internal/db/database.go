// The package db manages the creation and configuration of the SQLite database
// it uses Bun ORM with the driver go-sqlite3.
package db

import (
	"context"
	"database/sql"
	"encoding/hex"
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

	success = true

	// Register m2m models
	db.RegisterModel((*models.AlbumPhoto)(nil))
	db.RegisterModel((*models.PhotoFace)(nil))

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("error en migraciones: %w", err)
	}

	return db, nil
}

// addColumnIfMissing adds a column to an existing table when it is not there yet.
func addColumnIfMissing(ctx context.Context, db *bun.DB, table, column, definition string) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("reading columns of %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("reading columns of %s: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	if _, err := db.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition),
	); err != nil {
		return fmt.Errorf("adding %s.%s: %w", table, column, err)
	}
	return nil
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

	// CREATE TABLE IF NOT EXISTS does not touch tables that already exist, so
	// the columns added after the first release are added here.
	for _, c := range []struct{ table, column, definition string }{
		{"photos", "faces_scanned_at", "INTEGER NOT NULL DEFAULT 0"},
		{"faces", "samples", "INTEGER NOT NULL DEFAULT 0"},
		{"photo_faces", "embedding", "BLOB"},
		// SQLite lets a column with a NULL default be added together with its foreign key.
		{"faces", "cover_photo_id", `TEXT REFERENCES photos(id) ON DELETE SET NULL`},
	} {
		if err := addColumnIfMissing(ctx, db, c.table, c.column, c.definition); err != nil {
			return err
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

