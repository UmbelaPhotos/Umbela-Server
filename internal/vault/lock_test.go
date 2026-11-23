package vault

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"umbela-server/internal/config"
	"umbela-server/internal/db"
)

func TestLockUnlock(t *testing.T) {
	dir := t.TempDir()
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 1)
	}

	cfg, err := config.DefaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(kek); err != nil {
		t.Fatal(err)
	}
	dek := append([]byte(nil), cfg.Vault.DEK...)

	// The image processor is not needed here, so only the database is opened.
	v := NewVault(dir, &cfg.Vault)
	if v.db, err = db.InitDB(dir, cfg.Vault.DEK); err != nil {
		t.Fatal(err)
	}

	release, err := v.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	release()

	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Acquire(); !errors.Is(err, ErrLocked) {
		t.Fatalf("Acquire after Lock: got %v, want ErrLocked", err)
	}
	if v.config.DEK != nil {
		t.Fatal("DEK still in memory after Lock")
	}
	for _, ext := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(filepath.Join(dir, "database.db"+ext)); !os.IsNotExist(err) {
			t.Fatalf("database.db%s still there after Lock (err=%v)", ext, err)
		}
	}

	wrong := append([]byte(nil), kek...)
	wrong[0] ^= 0xff
	if err := v.Unlock(wrong); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("Unlock with wrong KEK: got %v, want ErrInvalidPassword", err)
	}
	if _, err := v.Acquire(); !errors.Is(err, ErrLocked) {
		t.Fatal("wrong KEK must leave the vault locked")
	}

	if err := v.Unlock(kek); err != nil {
		t.Fatal(err)
	}
	release, err = v.Acquire()
	if err != nil {
		t.Fatalf("Acquire after Unlock: %v", err)
	}
	release()
	if string(v.config.DEK) != string(dek) {
		t.Fatal("DEK not restored")
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
}
