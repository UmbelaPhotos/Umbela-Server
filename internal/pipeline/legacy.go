package pipeline
//   jobs := make(chan *FileGroup, len(groups))

//     results := make(chan Result, len(groups))

//     var wg sync.WaitGroup


//     const numWorkers = 8

//     for range numWorkers {

//         wg.Add(1)

//         go func() {

//             defer wg.Done()

//             for group := range jobs {

//                 photo, err := processGroup(ctx, group, processor, repo, photosDir, dryRun, move)

//                 results <- Result{Group: group, Photo: photo, Err: err}

//             }

//         }()

//     }


//     for _, g := range groups {

//         jobs <- g

//     }

//     close(jobs)

//     wg.Wait()

//     close(results) func processGroup(ctx context.Context, group *FileGroup, processor *image.Processor, repo *storage.PhotoRepository, photosDir string, dryRun, move bool) (*models.Photo, error) {

//     imagePath := pickBestImage(group.ImagePaths)

//     if imagePath == "" {

//         return nil, nil

//     }


//     hash, err := image.ComputeSHA256(imagePath)

//     if err != nil {

//         return nil, fmt.Errorf("calculando hash: %w", err)

//     }


//     existing, err := repo.GetByHash(ctx, hash)

//     if err != nil {

//         return nil, fmt.Errorf("verificando duplicado: %w", err)

//     }

//     if existing != nil {

//         return nil, nil

//     }


//     ext := strings.ToLower(filepath.Ext(imagePath))

//     destPath := filepath.Join(photosDir, hash+ext)


//     if !dryRun {

//         if err := os.MkdirAll(photosDir, 0755); err != nil {

//             return nil, fmt.Errorf("creando directorio destino: %w", err)

//         }

//         if move {

//             if err := os.Rename(imagePath, destPath); err != nil {

//                 // Rename falla entre distintos dispositivos; fallback a copy+delete

//                 if err2 := copyFile(imagePath, destPath); err2 != nil {

//                     return nil, fmt.Errorf("moviendo archivo: %w", err2)

//                 }

//                 os.Remove(imagePath)

//             }

//         } else {

//             if err := copyFile(imagePath, destPath); err != nil {

//                 return nil, fmt.Errorf("copiando archivo: %w", err)

//             }

//         }

//     }


//     var metadata *image.EXIFData

//     var metaErr error

//     if group.JSONPath != "" {

//         metadata, metaErr = image.ProcessMetadataJSON(group.JSONPath)

//     } else {

//         metadata, metaErr = image.ExtractEXIF(imagePath)

//     }

//     if metaErr != nil || metadata == nil {

//         metadata = &image.EXIFData{Title: filepath.Base(imagePath)}

//     }


//     photo := &models.Photo{

//         Hash:        hash,

//         Title:       metadata.Title,

//         Description: metadata.Description,

//         Artist:      metadata.Artist,

//         CapturedAt:  metadata.CapturedAt,

//         Latitude:    metadata.Latitude,

//         Longitude:   metadata.Longitude,

//         FilePath:    destPath,

//     }


//     if !dryRun {

//         thumbResult, err := processor.GenerateThumbnailAndBlurHash(destPath, hash)

//         if err != nil {

//             return nil, fmt.Errorf("generando thumbnail: %w", err)

//         }

//         photo.ThumbPath = thumbResult.ThumbPath

//         photo.Blurhash = thumbResult.Blurhash


//         if err := repo.Save(ctx, photo); err != nil {

//             return nil, fmt.Errorf("guardando en BD: %w", err)

//         }

//     }


//     return photo, nil

// } 