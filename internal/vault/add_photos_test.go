package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"umbela-server/internal/config"
)

// Importing a folder tree must leave one album per folder, with its photos,
// and the albums must show up in ListAlbums.
func TestAddPhotosCreatesAlbums(t *testing.T) {
	src := t.TempDir()
	for dst, from := range map[string]string{
		"loose.jpg":   "../../tests/photo/photo1.jpg",
		"Viaje/a.jpg": "../../tests/photo/photo2.jpg",
		"Viaje/b.jpg": "../../tests/photo/photo1.jpg", // same content as loose.jpg
		"Boda/c.jpg":  "../../tests/photo/photo2.jpg", // same content as Viaje/a.jpg
	} {
		raw, err := os.ReadFile(from)
		if err != nil {
			t.Skip("test photos not available")
		}
		raw = append(raw, []byte(dst)...) // make every file different
		p := filepath.Join(src, dst)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	home := t.TempDir()
	cfg := &config.VaultConfig{
		DEK:       make([]byte, 32),
		ImageDir:  filepath.Join(home, "images"),
		ThumbsDir: filepath.Join(home, "thumbs"),
	}
	v := NewVault(home, cfg)
	ctx := context.Background()
	if err := v.AddPhotos(ctx, src, true); err != nil {
		t.Fatal(err)
	}

	list, err := v.ListAlbums(ctx)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int32{}
	for _, a := range list {
		counts[a.Name] = a.PhotoCount
	}
	t.Logf("albums: %v", counts)
	if len(list) != 2 || counts["Viaje"] != 2 || counts["Boda"] != 1 {
		t.Fatalf("want Viaje=2 and Boda=1, got %v", counts)
	}
}
