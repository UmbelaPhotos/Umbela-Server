package storage

import (
	"context"
	"fmt"

	"umbela-server/internal/models"

	"github.com/uptrace/bun"
)

// FaceRepository encapsulates the operations of the face and person models.
type FaceRepository struct {
	db *bun.DB
}

func NewFaceRepository(db *bun.DB) *FaceRepository {
	return &FaceRepository{db: db}
}

// ScanResult is everything a face scan of one photo changes in the database.
type ScanResult struct {
	PhotoID       string
	NewPeople     []*models.Face      // people created by this scan
	UpdatedPeople []*models.Face      // existing people whose centroid changed
	Links         []*models.PhotoFace // one per face found in the photo
	ScannedAt     int64
}

// PendingPhotos returns up to limit photos that have not been scanned yet.
func (r *FaceRepository) PendingPhotos(ctx context.Context, limit int) ([]*models.Photo, error) {
	var photos []*models.Photo
	err := r.db.NewSelect().
		Model(&photos).
		Column("id", "file_path").
		Where("faces_scanned_at = 0").
		OrderExpr("created_at, id").
		Limit(int64(limit)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing photos pending a face scan: %w", err)
	}
	return photos, nil
}

// CountPending returns how many photos are waiting for a face scan.
func (r *FaceRepository) CountPending(ctx context.Context) (int, error) {
	n, err := r.db.NewSelect().Model((*models.Photo)(nil)).Where("faces_scanned_at = 0").Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting photos pending a face scan: %w", err)
	}
	return int(n), nil
}

// ListPeople returns every person with the centroid of their embeddings.
func (r *FaceRepository) ListPeople(ctx context.Context) ([]*models.Face, error) {
	var people []*models.Face
	if err := r.db.NewSelect().Model(&people).Column("id", "embedding", "samples").Scan(ctx); err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	return people, nil
}

// SaveScan stores the result of scanning a photo and marks the photo as
// scanned, all in one transaction so a stop in the middle cannot leave half
// of a scan behind.
func (r *FaceRepository) SaveScan(ctx context.Context, s ScanResult) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if len(s.NewPeople) > 0 {
			if _, err := tx.NewInsert().Model(&s.NewPeople).Exec(ctx); err != nil {
				return fmt.Errorf("inserting people: %w", err)
			}
		}
		for _, p := range s.UpdatedPeople {
			if _, err := tx.NewUpdate().Model(p).Column("embedding", "samples").WherePK().Exec(ctx); err != nil {
				return fmt.Errorf("updating person %s: %w", p.ID, err)
			}
		}
		if len(s.Links) > 0 {
			if _, err := tx.NewInsert().Model(&s.Links).On("CONFLICT (photo_id, face_id) DO NOTHING").Exec(ctx); err != nil {
				return fmt.Errorf("linking faces to photo %s: %w", s.PhotoID, err)
			}
		}
		if _, err := tx.NewUpdate().
			Model((*models.Photo)(nil)).
			Set("faces_scanned_at = ?", s.ScannedAt).
			Where("id = ?", s.PhotoID).
			Exec(ctx); err != nil {
			return fmt.Errorf("marking photo %s as scanned: %w", s.PhotoID, err)
		}
		return nil
	})
}
