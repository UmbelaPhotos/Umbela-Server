package vault

import (
	"context"
	"fmt"
	"os"
	"umbela-server/internal/ingestion"
	"umbela-server/internal/storage"
)

func (v *Vault) AddPhotos(ctx context.Context,addImagesPath string, lastDirIsAlbum bool) error{
	if err := v.SetUp(); err !=nil{
		return err
	}

	extractDir, err := os.MkdirTemp(addImagesPath, "umbela-extract-*")
	if err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	directoryFileGroups, err := ingestion.ScanDirectory(addImagesPath, extractDir, lastDirIsAlbum)
	if err != nil {
		return err
	}

	photoRepo := storage.NewPhotoRepository(v.db)

	ingestion.IngestPhotos(
		ctx,
		directoryFileGroups,
		v.imageProcessor,
		photoRepo,
		v.config,
		false,
		false,
	)
	return nil
}