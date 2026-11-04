package ingestion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectoryAlbums(t *testing.T) {
	root := t.TempDir()
	extract := filepath.Join(root, "extract")
	for _, f := range []string{"loose.jpg", "Viaje/a.jpg", "Viaje/b.png", "Boda/c.jpg"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	albums := func(lastDirIsAlbum bool) map[string]string {
		groups, err := ScanDirectory(root, extract, lastDirIsAlbum)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, g := range groups {
			out[filepath.Base(g.ImagePaths[0])] = g.Album
		}
		return out
	}

	got := albums(true)
	want := map[string]string{"loose.jpg": "", "a.jpg": "Viaje", "b.png": "Viaje", "c.jpg": "Boda"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: album %q, want %q", k, got[k], v)
		}
	}

	for k, v := range albums(false) {
		if v != "" {
			t.Errorf("--no-albums: %s still has album %q", k, v)
		}
	}
}
