package vault

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"sort"
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

const (
	// A face joins an existing person when it is at least this similar to the
	// person's centroid. It is the value SFace recommends to tell if two faces
	// are the same person; a stricter one split people between photos taken
	// with different light, age or angle.
	faceMatchThreshold = faces.SuggestThreshold

	// Two people whose centroids are at least this similar are merged into
	// one. Centroids average many faces, so they are less noisy than a single
	// face, but a merge cannot be undone, hence a bit above the match value.
	personMergeThreshold = 0.40

	// Pairs between these two values are reported in the log, to tune them.
	personNearMissFloor = 0.30

	// At most this many near misses are logged per pass.
	maxNearMissLogs = 30
)

// person is a group of faces of the same individual, as kept in memory.
type person struct {
	id       string
	name     string
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

	// People split by earlier scans (or by older thresholds) are joined first.
	_ = v.mergeSimilarPeople(ctx, repo)

	scanned := 0
	for {
		pending, err := repo.PendingPhotos(ctx, faceScanBatch)
		if err != nil {
			return err
		}

		if len(pending) == 0 {
			if scanned > 0 {
				scanned = 0
				_ = v.mergeSimilarPeople(ctx, repo)
				people = nil // merged people are gone: reload before the next scan
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(faceScanIdle):
				continue
			}
		}

		if engine == nil {
			engine, err = faces.NewEngine(faces.LibraryPath())
			if err != nil {
				return fmt.Errorf("starting the face engine: %w", err)
			}
		}
		if people == nil {
			var err error
			people, err = loadPeople(ctx, repo)
			if err != nil {
				return err
			}
		}

		for _, photo := range pending {
			if ctx.Err() != nil {
				return nil
			}

			result, _, err := v.scanPhoto(engine, people, photo)
			if err != nil {
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
		people = append(people, &person{id: f.ID, name: f.PersonName, centroid: emb, samples: f.Samples})
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
		if match != nil && sim >= faceMatchThreshold {
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

// mergeSimilarPeople joins the people that are the same individual: the
// bigger group keeps its id and takes the other's photos. People that appear
// together in a photo, or that have different names, are never merged.
func (v *Vault) mergeSimilarPeople(ctx context.Context, repo *storage.FaceRepository) error {
	people, err := loadPeople(ctx, repo)
	if err != nil {
		return err
	}
	// Biggest first, so the best known centroid absorbs the small ones.
	sort.SliceStable(people, func(i, j int) bool { return people[i].samples > people[j].samples })

	gone := make([]bool, len(people))
	merged := 0
	for i, a := range people {
		if gone[i] {
			continue
		}
		for j := i + 1; j < len(people); j++ {
			b := people[j]
			if gone[j] {
				continue
			}
			sim := faces.Similarity(a.centroid, b.centroid)
			if sim < personNearMissFloor || sim < personMergeThreshold {
				continue
			}
			if a.name != "" && b.name != "" && a.name != b.name {
				continue // the user told them apart
			}
			centroid := weightedCentroid(a, b)
			err := repo.MergePeople(ctx, a.id, b.id, faces.EncodeEmbedding(centroid), a.samples+b.samples)
			if errors.Is(err, storage.ErrPeopleShareAPhoto) {
				continue
			}
			if err != nil {
				return err
			}
			a.centroid, a.samples = centroid, a.samples+b.samples
			if a.name == "" {
				a.name = b.name
			}
			gone[j] = true
			merged++
		}
	}
	return nil
}

// weightedCentroid is the normalized mean of the faces of two groups.
func weightedCentroid(a, b *person) []float32 {
	out := make([]float32, len(a.centroid))
	var norm float64
	for i := range out {
		out[i] = a.centroid[i]*float32(a.samples) + b.centroid[i]*float32(b.samples)
		norm += float64(out[i]) * float64(out[i])
	}
	if norm = math.Sqrt(norm); norm > 0 {
		for i := range out {
			out[i] = float32(float64(out[i]) / norm)
		}
	}
	return out
}
