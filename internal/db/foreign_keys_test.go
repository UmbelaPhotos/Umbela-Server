package db

import (
	"context"
	"testing"
)

// The cover of an album or person has to be a photo that exists, and the
// database clears it by itself when that photo is deleted.
func TestCoverPhotoIsARealForeignKey(t *testing.T) {
	ctx := context.Background()
	database, err := InitDB(t.TempDir(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := database.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO photos (id, hash, file_path) VALUES ('p1', 'h1', 'x')`)

	for _, table := range []string{"albums", "faces"} {
		insert := map[string]string{
			"albums": `INSERT INTO albums (id, name, cover_photo_id) VALUES (?, 'a', ?)`,
			"faces":  `INSERT INTO faces (id, embedding, cover_photo_id) VALUES (?, x'00', ?)`,
		}[table]

		// Negative test: a cover that is not a photo is refused.
		if _, err := database.ExecContext(ctx, insert, table+"-bad", "no-such-photo"); err == nil {
			t.Errorf("%s: a cover pointing to a missing photo was accepted", table)
		}
		// No cover (NULL) and a real cover are fine.
		mustExec(insert, table+"-none", nil)
		mustExec(insert, table+"-ok", "p1")
	}

	// Deleting the photo leaves the covers empty instead of dangling.
	mustExec(`DELETE FROM photos WHERE id = 'p1'`)
	for _, table := range []string{"albums", "faces"} {
		var left int
		q := `SELECT COUNT(*) FROM ` + table + ` WHERE cover_photo_id IS NOT NULL`
		if err := database.QueryRowContext(ctx, q).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left != 0 {
			t.Errorf("%s: %d covers still point to the deleted photo", table, left)
		}
	}
}

// A database made before faces had a cover gets the column with its foreign key.
func TestOldDatabaseGetsTheFacesCoverForeignKey(t *testing.T) {
	// Reuses the old schema of migrate_test.go: its faces table has no cover.
	dir := t.TempDir()
	key := make([]byte, 32)
	openOld(t, dir, key)

	database, err := InitDB(dir, key)
	if err != nil {
		t.Fatalf("opening the old database: %v", err)
	}
	defer database.Close()

	rows, err := database.Query(`PRAGMA foreign_key_list(faces)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatal(err)
		}
		if from == "cover_photo_id" && table == "photos" && onDelete == "SET NULL" {
			found = true
		}
	}
	if !found {
		t.Error("faces.cover_photo_id is not a foreign key to photos(id) ON DELETE SET NULL")
	}
}
