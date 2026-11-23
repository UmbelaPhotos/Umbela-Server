package vault

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"umbela-server/internal/config"
)

// An upload is stored once; the same bytes again are reported as duplicates
// and MissingHashes stops listing them.
func TestUploadPhoto(t *testing.T) {
	raw, err := os.ReadFile("../../tests/photo/photo1.jpg")
	if err != nil {
		t.Skip("test photos not available")
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])

	home := t.TempDir()
	v := NewVault(home, &config.VaultConfig{
		DEK:       make([]byte, 32),
		ImageDir:  filepath.Join(home, "images"),
		ThumbsDir: filepath.Join(home, "thumbs"),
	})
	if err := v.SetUp(); err != nil {
		t.Fatal(err)
	}
	defer v.ShutDown()
	ctx := context.Background()

	missing, err := v.MissingHashes(ctx, []string{hash})
	if err != nil || len(missing) != 1 {
		t.Fatalf("want hash missing, got %v %v", missing, err)
	}

	id, dup, err := v.UploadPhoto(ctx, raw, "../../evil/IMG_1.JPG", 1700000000)
	if err != nil || dup || id == "" {
		t.Fatalf("first upload: id=%q dup=%v err=%v", id, dup, err)
	}
	if _, dup, err = v.UploadPhoto(ctx, raw, "IMG_1.jpg", 0); err != nil || !dup {
		t.Fatalf("second upload: dup=%v err=%v", dup, err)
	}
	if missing, _ = v.MissingHashes(ctx, []string{hash}); len(missing) != 0 {
		t.Fatalf("hash still missing: %v", missing)
	}
	if _, _, err = v.UploadPhoto(ctx, raw, "notes.txt", 0); !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("want ErrUnsupportedFile, got %v", err)
	}
}
