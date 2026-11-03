package vault

import (
	"context"
	"fmt"
	"image"
	"os"
	"time"
	"uuid"

	"umbela-server/internal/crypto"
	"umbela-server/internal/faces"
	"umbela-server/internal/models"
	"umbela-server/internal/storage"

	"github.com/davidbyttow/govips/v2/vips"
)

const (
	// Photos are scanned at this size at most. The detector works on a
	// smaller picture anyway; this keeps enough detail to crop each face.
	faceScanMaxSide = 1920
	faceScanBatch   = 20
	faceScanIdle    = 30 * time.Second
)

// person is a group of faces of the same individual, as kept in memory.
type person struct {
	id       string
	centroid []float32
	samples  int
}

// RunFaceScanner looks for photos that have not been scanned for faces and
// scans them one by one until ctx is cancelled. It also picks up photos added
// while it runs. The vault must be set up before.
//
// If the face engine cannot start (for example the ONNX Runtime library is
// missing) it returns an error and the rest of the server keeps working.
func (v *Vault) RunFaceScanner(ctx context.Context) error {
	repo := storage.NewFaceRepository(v.db)

	var engine *faces.Engine
	var people []*person
	defer func() {
		if engine != nil {
			engine.Close()
		}
	}()

	scanned, found := 0, 0
	for {
		pending, err := repo.PendingPhotos(ctx, faceScanBatch)
		if err != nil {
			return err
		}

		if len(pending) == 0 {
			if scanned > 0 {
				fmt.Printf("Face scan finished: %d photos, %d faces\n", scanned, found)
				scanned, found = 0, 0
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(faceScanIdle):
				continue
			}
		}

		if engine == nil {
			total, _ := repo.CountPending(ctx)
			fmt.Printf("Face scan: %d photos pending\n", total)

			engine, err = faces.NewEngine(faces.LibraryPath())
			if err != nil {
				return fmt.Errorf("starting the face engine: %w", err)
			}
			people, err = loadPeople(ctx, repo)
			if err != nil {
				return err
			}
		}

		for _, photo := range pending {
			if ctx.Err() != nil {
				return nil
			}

			result, n, err := v.scanPhoto(engine, people, photo)
			if err != nil {
				fmt.Printf("Face scan: skipping photo %s: %v\n", photo.ID, err)
				// Mark it as scanned so a damaged file is not retried forever.
				result = storage.ScanResult{}
			}
			result.PhotoID = photo.ID
			result.ScannedAt = time.Now().Unix()

			if err := repo.SaveScan(ctx, result); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			// Only after the database accepted it, the memory follows.
			people = applyToPeople(people, result)

			scanned++
			found += n
			if scanned%50 == 0 {
				fmt.Printf("Face scan: %d photos done\n", scanned)
			}
		}
	}
}

func loadPeople(ctx context.Context, repo *storage.FaceRepository) ([]*person, error) {
	stored, err := repo.ListPeople(ctx)
	if err != nil {
		return nil, err
	}
	people := make([]*person, 0, len(stored))
	for _, f := range stored {
		emb, err := faces.DecodeEmbedding(f.Embedding)
		if err != nil || len(emb) != faces.EmbeddingSize {
			continue
		}
		people = append(people, &person{id: f.ID, centroid: emb, samples: f.Samples})
	}
	return people, nil
}

// scanPhoto detects the faces of a photo and decides which person each one is.
// It does not touch the database or the in-memory people: it returns what has
// to be saved. The int is the number of faces found.
func (v *Vault) scanPhoto(engine *faces.Engine, people []*person, photo *models.Photo) (storage.ScanResult, int, error) {
	var result storage.ScanResult

	encrypted, err := os.ReadFile(photo.FilePath)
	if err != nil {
		return result, 0, fmt.Errorf("reading photo: %w", err)
	}
	plain, err := crypto.DecryptGCM(encrypted, v.config.DEK)
	if err != nil {
		return result, 0, fmt.Errorf("decrypting photo: %w", err)
	}
	defer crypto.CleanBytes(plain)

	img, err := decodeForFaces(plain)
	if err != nil {
		return result, 0, err
	}
	found, err := engine.Analyze(img)
	if err != nil {
		return result, 0, err
	}

	bounds := img.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())

	// A person cannot appear twice in the same photo, so once a face took a
	// person, the next faces have to look for somebody else.
	taken := map[string]bool{}
	for _, f := range found {
		match, sim := bestPerson(people, f.Embedding, taken)

		var personID string
		if match != nil && sim >= faces.MatchThreshold {
			personID = match.id
			taken[personID] = true
			result.UpdatedPeople = append(result.UpdatedPeople, &models.Face{
				ID:        match.id,
				Embedding: faces.EncodeEmbedding(faces.Merge(match.centroid, match.samples, f.Embedding)),
				Samples:   match.samples + 1,
			})
		} else {
			personID = uuid.NewV7().String()
			result.NewPeople = append(result.NewPeople, &models.Face{
				ID:        personID,
				Embedding: faces.EncodeEmbedding(f.Embedding),
				Samples:   1,
			})
		}

		result.Links = append(result.Links, &models.PhotoFace{
			PhotoID:    photo.ID,
			FaceID:     personID,
			BBoxMinX:   clamp01(float64(f.Box.X1) / w),
			BBoxMinY:   clamp01(float64(f.Box.Y1) / h),
			BBoxMaxX:   clamp01(float64(f.Box.X2) / w),
			BBoxMaxY:   clamp01(float64(f.Box.Y2) / h),
			Confidence: float64(f.Score),
			Embedding:  faces.EncodeEmbedding(f.Embedding),
		})
	}
	return result, len(found), nil
}

// bestPerson returns the most similar person that is not in skip.
func bestPerson(people []*person, emb []float32, skip map[string]bool) (*person, float32) {
	var best *person
	var bestSim float32 = -2
	for _, p := range people {
		if skip[p.id] {
			continue
		}
		if s := faces.Similarity(p.centroid, emb); s > bestSim {
			best, bestSim = p, s
		}
	}
	return best, bestSim
}

// applyToPeople brings the in-memory people up to date with a saved scan.
func applyToPeople(people []*person, r storage.ScanResult) []*person {
	for _, u := range r.UpdatedPeople {
		emb, err := faces.DecodeEmbedding(u.Embedding)
		if err != nil {
			continue
		}
		for _, p := range people {
			if p.id == u.ID {
				p.centroid, p.samples = emb, u.Samples
				break
			}
		}
	}
	for _, n := range r.NewPeople {
		emb, err := faces.DecodeEmbedding(n.Embedding)
		if err != nil {
			continue
		}
		people = append(people, &person{id: n.ID, centroid: emb, samples: n.Samples})
	}
	return people
}

// decodeForFaces decodes a photo, applies its EXIF orientation and reduces it
// to at most faceScanMaxSide pixels.
func decodeForFaces(data []byte) (image.Image, error) {
	ref, err := vips.LoadThumbnailFromBuffer(data, faceScanMaxSide, faceScanMaxSide, vips.InterestingNone, vips.SizeDown, nil)
	if err != nil {
		return nil, fmt.Errorf("decoding photo: %w", err)
	}
	defer ref.Close()

	img, err := ref.ToGoImage()
	if err != nil {
		return nil, fmt.Errorf("converting photo: %w", err)
	}
	return img, nil
}

func clamp01(v float64) float64 {
	return max(0, min(1, v))
}
