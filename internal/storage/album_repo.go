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
        Returning("NONE"). 
        Exec(ctx)
    return err
}
func (r *AlbumRepository) GetByID(ctx context.Context, id int64) (*models.Album, error){
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

func (r *AlbumRepository) Delete(ctx context.Context, id int64) error{
	result, err := r.db.NewDelete().Model((*models.Album)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
    	return fmt.Errorf("deleting album %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("album %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}