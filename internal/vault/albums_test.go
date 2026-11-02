package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"umbela-server/internal/models"
)

func TestAlbumsAndDelete(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	photos := []models.Photo{
		{ID: "p1", Hash: "h1", FilePath: write("p1.umb"), ThumbPath: write("p1t.umb"), CapturedAt: 100},
		{ID: "p2", Hash: "h2", FilePath: write("p2.umb"), ThumbPath: write("p2t.umb"), CapturedAt: 200},
		{ID: "p3", Hash: "h3", FilePath: write("p3.umb"), CapturedAt: 300},
	}
	v := seededVault(t, photos)
	ctx := context.Background()

	if _, err := v.CreateAlbum(ctx, "  "); !errors.Is(err, ErrInvalidAlbumName) {
		t.Fatalf("blank name: got %v", err)
	}
	album, err := v.CreateAlbum(ctx, "Viaje")
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"p1", "p2", "p2"} { // twice: not an error
		if err := v.AddPhotoToAlbum(ctx, id, album.Id); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.AddPhotoToAlbum(ctx, "nope", album.Id); !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("missing photo: got %v", err)
	}
	if err := v.AddPhotoToAlbum(ctx, "p1", "nope"); !errors.Is(err, ErrAlbumNotFound) {
		t.Fatalf("missing album: got %v", err)
	}

	list, err := v.ListAlbums(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].PhotoCount != 2 || list[0].CoverPhotoId != "p2" {
		t.Fatalf("list = %+v", list)
	}

	page, _, err := v.GetAlbumTimelinePage(ctx, album.Id, 0, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Id != "p2" || page[1].Id != "p1" {
		t.Fatalf("album page = %+v", page)
	}
	all, _, _ := v.GetTimelinePage(ctx, 0, 0, 0, "")
	if len(all) != 3 {
		t.Fatalf("whole library = %d, want 3", len(all))
	}

	if err := v.DeletePhoto(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"p2.umb", "p2t.umb"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still on disk", f)
		}
	}
	if err := v.DeletePhoto(ctx, "p2"); !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("second delete: got %v", err)
	}
	list, _ = v.ListAlbums(ctx)
	if list[0].PhotoCount != 1 || list[0].CoverPhotoId != "p1" {
		t.Fatalf("after delete: %+v", list[0])
	}
}
