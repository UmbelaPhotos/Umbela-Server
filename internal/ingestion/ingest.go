package ingestion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/image"
	"umbela-server/internal/models"
	"umbela-server/internal/storage"

	"github.com/schollz/progressbar/v3"
)

type Result struct {
	Group *FileGroup
	Photo *models.Photo
	Err   error
}

// IngestPhotos orchestrate the concurrent processing pipeline.
func IngestPhotos(ctx context.Context, groups []*FileGroup, processor *image.Processor, repo *storage.PhotoRepository, config *config.VaultConfig, dryRun bool, move bool) {
	fmt.Println("Starting ingest")
	total := len(groups)
	jobs := make(chan *FileGroup, 100)
	results := make(chan Result, 100)

	var wg sync.WaitGroup
	const numWorkers = 8

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for group := range jobs {
				if ctx.Err() != nil {
					return
				}
				photo, err := processGroup(ctx, group, processor, repo, config.ImageDir, dryRun, move, config.DEK)
				results <- Result{Group: group, Photo: photo, Err: err}
			}
		}()
	}

	go func() {
		for _, g := range groups {
			jobs <- g
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true), 
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("photos"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetDescription("Ingesting photos"),
		progressbar.OptionSetTheme(progressbar.Theme{
        Saucer:        "[magenta]=[reset]",
        SaucerHead:    "[magenta]>[reset]",
        SaucerPadding: " ",
        BarStart:      "[",
        BarEnd:        "]",
    	}))

	var processed, skipped, failed int
	var errLogs []string
	for res := range results {
		_ = bar.Add(1)
		if res.Err != nil {
			errLogs = append(errLogs, fmt.Sprintf("error processing %s: %v", res.Group.ImagePaths[0], res.Err))
			failed++
		} else if res.Photo == nil {
			skipped++
		} else {
			processed++
		}
		bar.Describe(fmt.Sprintf("[white]Ingesting[reset] ([green]%dP[reset]/[dark_gray]%dS[reset]/[light_red]%dF[reset])", processed, skipped, failed))
	}

	fmt.Println("\n") 

	if len(errLogs) > 0 {
		for _, errStr := range errLogs {
			fmt.Printf("%s\n", errStr)
		}
	}

	fmt.Printf("Ingest finalized: %d Processed, %d Duplicates/Skip, %d Errors\n", processed, skipped, failed)
}

// processGroup life cycle of a photo being processed
func processGroup(ctx context.Context, group *FileGroup, processor *image.Processor, repo *storage.PhotoRepository, imagesDir string, dryRun, move bool, DEK []byte) (*models.Photo, error) {
	imagePath := group.ImagePaths[0]
	if imagePath == "" {
		return nil, nil
	}

	hash, err := crypto.ComputeSHA256(imagePath)
	if err != nil {
		return nil, fmt.Errorf("calculando hash: %w", err)
	}

	existing, err := repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("verificando duplicado: %w", err)
	}
	if existing != nil {
		return nil, nil
	}

	var metadata *image.EXIFData
	var metaErr error
	if group.JSONPath != "" {
		metadata, metaErr = image.ProcessMetadataJSON(group.JSONPath)
	} else {
		metadata, metaErr = image.ExtractEXIF(imagePath)
	}

	if metaErr != nil || metadata == nil {
		metadata = &image.EXIFData{Title: filepath.Base(imagePath)}
	}

	destPath := filepath.Join(imagesDir, hash+".umb")

	photo := &models.Photo{
		Hash:        hash,
		Title:       metadata.Title,
		Description: metadata.Description,
		Artist:      metadata.Artist,
		CapturedAt:  metadata.CapturedAt,
		Latitude:    metadata.Latitude,
		Longitude:   metadata.Longitude,
		FilePath:    destPath,
	}

	if !dryRun {
		if err := os.MkdirAll(imagesDir, 0700); err != nil {
			return nil, fmt.Errorf("dreating target directory: %w", err)
		}

		thumbResult, err := processor.GenerateThumbnailAndBlurHash(imagePath, hash, DEK)
		if err != nil {
			return nil, fmt.Errorf("generating thumbnail: %w", err)
		}
		photo.ThumbPath = thumbResult.ThumbPath
		photo.Blurhash = thumbResult.Blurhash

		plainBytes, err := os.ReadFile(imagePath)
		if err != nil {
			return nil, fmt.Errorf("reading original image: %w", err)
		}

		if err := processor.SaveImage(destPath, plainBytes, DEK); err != nil {
			return nil, fmt.Errorf("saving encrypted original image: %w", err)
		}

		// Eliminate original if user choose move
		if move {
			os.Remove(imagePath)
		}

		// Saves to db
		if err := repo.Save(ctx, photo); err != nil {
			return nil, fmt.Errorf("guardando en BD: %w", err)
		}
	}

	return photo, nil
}
