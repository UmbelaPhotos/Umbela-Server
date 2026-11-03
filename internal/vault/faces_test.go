package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/models"

	"github.com/davidbyttow/govips/v2/vips"
)

// TestFaceScanner runs the real models over two photos. It needs:
//
//	UMBELA_ORT_LIB     path to libonnxruntime.so
//	UMBELA_TEST_PHOTO  a photo with faces
//	UMBELA_TEST_PHOTO2 another photo, with at least one person from the first
func TestFaceScanner(t *testing.T) {
	lib := os.Getenv("UMBELA_ORT_LIB")
	paths := []string{os.Getenv("UMBELA_TEST_PHOTO"), os.Getenv("UMBELA_TEST_PHOTO2")}
	if lib == "" || paths[0] == "" || paths[1] == "" {
		t.Skip("set UMBELA_ORT_LIB, UMBELA_TEST_PHOTO and UMBELA_TEST_PHOTO2 to run")
	}

	if err := vips.Startup(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vips.Shutdown)

	dek := make([]byte, 32)
	dir := t.TempDir()
	var photos []models.Photo
	for i, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		enc, err := crypto.EncryptGCM(raw, dek)
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, "photo"+string(rune('1'+i))+".umb")
		if err := os.WriteFile(dst, enc, 0600); err != nil {
			t.Fatal(err)
		}
		photos = append(photos, models.Photo{
			ID:       "p" + string(rune('1'+i)),
			Hash:     "h" + string(rune('1'+i)),
			FilePath: dst,
		})
	}

	v := seededVault(t, photos)
	v.config = &config.VaultConfig{DEK: dek}
	t.Setenv("UMBELA_ORT_LIB", lib)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- v.RunFaceScanner(ctx) }()

	deadline := time.Now().Add(2 * time.Minute)
	for {
		n, err := v.db.NewSelect().Model((*models.Photo)(nil)).Where("faces_scanned_at = 0").Count(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("scanner stopped early: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for the scan")
		}
		time.Sleep(200 * time.Millisecond)
	}
	cancel()
	<-done

	bg := context.Background()
	links, err := v.db.NewSelect().Model((*models.PhotoFace)(nil)).Count(bg)
	if err != nil {
		t.Fatal(err)
	}
	people, err := v.db.NewSelect().Model((*models.Face)(nil)).Count(bg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d faces found, %d people", links, people)
	if links == 0 {
		t.Fatal("no faces were stored")
	}
	if people >= links {
		t.Fatalf("expected at least one person to appear in both photos, got %d people for %d faces", people, links)
	}

	var shared int
	err = v.db.NewRaw(`SELECT COUNT(*) FROM (
		SELECT face_id FROM photo_faces GROUP BY face_id HAVING COUNT(DISTINCT photo_id) > 1)`).Scan(bg, &shared)
	if err != nil {
		t.Fatal(err)
	}
	if shared == 0 {
		t.Fatal("no person was matched across the two photos")
	}

	// With only two photos nobody reaches the minimum, so the list is empty,
	// but the person in both photos is there with all of their photos.
	listed, err := v.ListPeople(bg)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("nobody has %d photos yet, got %v", minPhotosToListPerson, listed)
	}

	var sharedID string
	err = v.db.NewRaw(`SELECT face_id FROM photo_faces GROUP BY face_id
		HAVING COUNT(DISTINCT photo_id) > 1 LIMIT 1`).Scan(bg, &sharedID)
	if err != nil {
		t.Fatal(err)
	}
	details, err := v.GetPersonDetails(bg, sharedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Photos) != 2 || details.Person.CoverPhotoId == "" {
		t.Fatalf("the person in both photos should open with 2 photos and a cover: %v", details)
	}
	t.Logf("person %s: %d photos, cover %s", sharedID, len(details.Photos), details.Person.CoverPhotoId)
}
