// Package storage manages the data access (CRUD) for the models of the domain.
// Every file in this package has a repository for the specific model.
// This repositories are the only ones that talk to bun
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"allium-server/internal/models"

	"github.com/uptrace/bun"
)

// PhotoRepository encapsulates all the operations of the photo model.
type PhotoRepository struct {
	db *bun.DB
}

// NewPhotoRepository creates a new instance of the Photo repository.
//
// Input:  *bun.DB — connection to the database
// Output: *PhotoRepository ready to use
func NewPhotoRepository(db *bun.DB) *PhotoRepository {
	return &PhotoRepository{db: db}
}

// Save saves a new Photo in the DB.If there already exists a photo with the same 
// Hash (deduplication), returns the instance without an error.
//
// Input:  ctx context.Context, photo *models.Photo with all required fields
// Output: error if the insert fails for another unexpected reason
func (r *PhotoRepository) Save(ctx context.Context, photo *models.Photo) error {
	_, err := r.db.NewInsert().Model(photo).On("CONFLICT (hash) DO NOTHING").Exec(ctx)
	return err
}

// GetByID searches a Photo by its id.
//
// Input:  ctx context.Context, id int64
// Output: *models.Photo found, or error if it doesnt exists / error in the BD
func (r *PhotoRepository) GetByID(ctx context.Context, id int64) (*models.Photo, error) {
	photo := new(models.Photo)
	err := r.db.NewSelect().Model(photo).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { 
			return nil, fmt.Errorf("photo with id %d not found: %w", id, err)
		}
		return nil, fmt.Errorf("trying to get the image by id: %w", err)
	}
	return photo, nil
}

// GetByHash searchs an image by its SHA256 (for deduplication).
//
// Input:  ctx context.Context, hash string — SHA256 hexadecimal of the original image
// Output: *models.Photo if it exists, nil and nil if it doesnt
func (r *PhotoRepository) GetByHash(ctx context.Context, hash string) (*models.Photo, error) {
	photo := new(models.Photo)
	err := r.db.NewSelect().Model(photo).Where("hash = ?", hash).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return photo, nil
}

// ListPaginated returns a page of photos with descending order.
//
// Input:  ctx, limit int (Photos per page), offset int (skip)
// Output: slice de fotos, total of photos in the BD, error
func (r *PhotoRepository) ListPaginated(ctx context.Context, limit int, offset int) ([]*models.Photo, int, error) {
	photos := []*models.Photo{}
	total, err := r.db.NewSelect().Model(&photos).OrderExpr("captured_at DESC").Limit(limit).Offset(offset).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("getting paginated images: %w", err)
	}
	return photos, total, nil
}

// Delete a photo from the DB by its ID.
// NOTE: It doesnt delete the file; that is responsability of the caller.
//
// Input:  ctx, id int64
// Output: error if it doesnt exists or crash in the BD
func (r *PhotoRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.NewDelete().Model((*models.Photo)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting photo %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("photo %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}
