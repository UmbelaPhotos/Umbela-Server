package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/ingestion"
	"umbela-server/internal/storage"
)

func (v *Vault) AddPhotos(ctx context.Context,addImagesPath string) error{
	if err := v.SetUp(); err !=nil{
		return err
	}
	
	tempDir := filepath.Join(v.path, "tmp")
	err := os.MkdirAll(tempDir, 0700)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Created temporal dir for extraction: %v\n", tempDir)

	directoryFileGroups, err := ingestion.ScanDirectory(addImagesPath, 4, tempDir)
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