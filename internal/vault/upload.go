package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"umbela-server/internal/ingestion"
	"umbela-server/internal/storage"
)

// ErrUnsupportedFile is returned for uploads that are not a supported image.
var ErrUnsupportedFile = errors.New("unsupported file type")

var uploadExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".heic": true, ".webp": true}

// MissingHashes returns the hashes the vault does not have yet.
func (v *Vault) MissingHashes(ctx context.Context, hashes []string) ([]string, error) {
	repo := storage.NewPhotoRepository(v.db)
	var missing []string
	for _, h := range hashes {
		existing, err := repo.GetByHash(ctx, h)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			missing = append(missing, h)
		}
	}
	return missing, nil
}

// UploadPhoto ingests one file sent by the phone. duplicate is true when the
// vault already had it. The caller must hold the vault (Acquire).
func (v *Vault) UploadPhoto(ctx context.Context, data []byte, filename string, capturedAt int64) (id string, duplicate bool, err error) {
	// Only the extension is kept: the name could try to escape the temp dir.
	ext := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	if !uploadExts[ext] {
		return "", false, ErrUnsupportedFile
	}

	dir, err := os.MkdirTemp("", "umbela-upload-*")
	if err != nil {
		return "", false, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "upload"+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", false, fmt.Errorf("writing upload: %w", err)
	}

	photo, err := ingestion.IngestFile(ctx, path, capturedAt, v.imageProcessor,
		storage.NewPhotoRepository(v.db), storage.NewAlbumRepository(v.db), v.config)
	if err != nil {
		return "", false, err
	}
	if photo == nil {
		return "", true, nil
	}
	return photo.ID, false, nil
}
