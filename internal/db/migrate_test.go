package db

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"
)

// The schema as it was before the face scan existed.
var oldSchema = []string{
	`CREATE TABLE photos (id TEXT PRIMARY KEY, hash TEXT NOT NULL UNIQUE, title TEXT, description TEXT,
		artist TEXT, width INTEGER, Height INTEGER, captured_at INTEGER, latitude REAL, longitude REAL,
		file_path TEXT NOT NULL, thumb_path TEXT, blurhash TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, favorite BOOLEAN DEFAULT FALSE)`,
	`CREATE TABLE faces (id TEXT PRIMARY KEY, person_name TEXT, embedding BLOB NOT NULL)`,
	`CREATE TABLE photo_faces (photo_id TEXT NOT NULL, face_id TEXT NOT NULL, bbox_min_x REAL NOT NULL,
		bbox_min_y REAL NOT NULL, bbox_max_x REAL NOT NULL, bbox_max_y REAL NOT NULL, confidence REAL,
		PRIMARY KEY (photo_id, face_id))`,
	`INSERT INTO photos (id, hash, file_path) VALUES ('old', 'h', 'x')`,
}

// A database created before the face columns existed must get them on open,
// and keep its photos.
func TestMigrationAddsFaceColumnsToOldDatabase(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)

	dsn := fmt.Sprintf("%s?_pragma_key=x'%s'&_pragma_cipher_page_size=4096",
		filepath.Join(dir, "database.db"), hex.EncodeToString(key))
	old, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range oldSchema {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	old.Close()

	// Open it twice: the second time everything is already there.
	for range 2 {
		database, err := InitDB(dir, key)
		if err != nil {
			t.Fatalf("opening the old database: %v", err)
		}

		for _, c := range []struct{ table, column string }{
			{"photos", "faces_scanned_at"},
			{"faces", "samples"},
			{"photo_faces", "embedding"},
		} {
			var n int
			q := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", c.table, c.column)
			if err := database.QueryRow(q).Scan(&n); err != nil || n != 1 {
				t.Errorf("%s.%s missing (count=%d, err=%v)", c.table, c.column, n, err)
			}
		}

		var scannedAt int64
		if err := database.QueryRow(`SELECT faces_scanned_at FROM photos WHERE id = 'old'`).Scan(&scannedAt); err != nil {
			t.Fatalf("the old photo is gone: %v", err)
		}
		if scannedAt != 0 {
			t.Errorf("an old photo should be pending a scan, got faces_scanned_at=%d", scannedAt)
		}
		database.Close()
	}
}
