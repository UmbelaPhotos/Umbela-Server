package vault

import (
	"context"
	"umbela-server/internal/ingestion"
	"umbela-server/internal/storage"
)

func (v *Vault) AddPhotos(ctx context.Context,addImagesPath string, lastDirIsAlbum bool) error{
	if err := v.SetUp(); err !=nil{
		return err
	}

	directoryFileGroups, err := ingestion.ScanDirectory(addImagesPath, lastDirIsAlbum)
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