package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"umbela-server/internal/models"

	"github.com/uptrace/bun"
)

type AlbumRepository struct{
	db *bun.DB
}

func NewAlbumRepository(db *bun.DB) *AlbumRepository{
	return &AlbumRepository{db:db}
}

func (r *AlbumRepository) Save(ctx context.Context, album *models.Album) error {
    if album.ID == "" {
        id := uuid.NewV7()
        album.ID = id.String()
    }
	_, err := r.db.NewInsert().
        Model(album).
        Returning(""). 
        Exec(ctx)
    return err
}
func (r *AlbumRepository) GetByID(ctx context.Context, id string) (*models.Album, error){
	album := models.Album{}
	err := r.db.NewSelect().Model(&album).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows){
			return nil, fmt.Errorf("no album with that id. %w", err)
		}
		return nil, fmt.Errorf("trying to get the album by id: %w", err) 
	}

	return &album, nil 
}

func (r *AlbumRepository) ListPaginated(ctx context.Context, limit int64, offset int64)([]*models.Album, int64, error){
	albums := []*models.Album{}
	total, err := r.db.NewSelect().Model(&albums).OrderExpr("updated_at DESC").Limit(limit).Offset(offset).ScanAndCount(ctx)
	if err != nil{
		return nil, 0, fmt.Errorf("Error getting the albums: %w", err)
	}

	return albums, total, nil
}

func (r *AlbumRepository) Delete(ctx context.Context, id string) error{
	result, err := r.db.NewDelete().Model((*models.Album)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
    	return fmt.Errorf("deleting album %s: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("album %s not found: %w", id, sql.ErrNoRows)
	}
	return nil
}

// AlbumRow is an album as the list shows it.
type AlbumRow struct {
	ID           string `bun:"id"`
	Name         string `bun:"name"`
	PhotoCount   int    `bun:"photo_count"`
	CoverPhotoID string `bun:"cover_photo_id"`
}

// ListWithCounts returns every album, most recently updated first, with its
// number of photos. A album without an explicit cover uses its newest photo.
func (r *AlbumRepository) ListWithCounts(ctx context.Context) ([]AlbumRow, error) {
	var rows []AlbumRow
	err := r.db.NewSelect().
		TableExpr("albums AS a").
		ColumnExpr("a.id AS id, a.name AS name").
		ColumnExpr("(SELECT COUNT(*) FROM album_photos ap WHERE ap.album_id = a.id) AS photo_count").
		ColumnExpr(`COALESCE(NULLIF(a.cover_photo_id, ''), (
			SELECT ap.photo_id FROM album_photos ap
			JOIN photos p ON p.id = ap.photo_id
			WHERE ap.album_id = a.id
			ORDER BY p.captured_at DESC, p.id DESC LIMIT 1
		), '') AS cover_photo_id`).
		OrderExpr("a.updated_at DESC, a.id DESC").
		Scan(ctx, &rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("listing albums: %w", err)
	}
	return rows, nil
}

// AddPhoto puts a photo in an album. Adding it twice is not an error.
func (r *AlbumRepository) AddPhoto(ctx context.Context, albumID, photoID string) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		link := &models.AlbumPhoto{AlbumID: albumID, PhotoID: photoID}
		if _, err := tx.NewInsert().Model(link).On("CONFLICT DO NOTHING").Exec(ctx); err != nil {
			return fmt.Errorf("adding photo %s to album %s: %w", photoID, albumID, err)
		}
		_, err := tx.NewUpdate().Model((*models.Album)(nil)).
			Set("updated_at = CURRENT_TIMESTAMP").
			Where("id = ?", albumID).
			Exec(ctx)
		return err
	})
}
