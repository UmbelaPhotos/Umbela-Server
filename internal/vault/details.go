package vault

import (
	"context"
	"database/sql"
	"errors"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/models"
)

// GetPhotoDetails returns what the viewer's info sheet shows for one photo.
func (v *Vault) GetPhotoDetails(ctx context.Context, photoID string) (*pb.PhotoDetails, error) {
	var p models.Photo
	err := v.db.NewSelect().Model(&p).Where("id = ?", photoID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPhotoNotFound
	}
	if err != nil {
		return nil, err
	}

	return &pb.PhotoDetails{
		Summary: &pb.PhotoSummary{
			Id:          p.ID,
			TakenAtUnix: p.CapturedAt,
			Blurhash:    p.Blurhash,
			Width:       int32(p.Width),
			Height:      int32(p.Height),
			Favorite:    p.Favorite,
		},
		Uploader:        p.Artist,
		Latitude:        p.Latitude,
		Longitude:       p.Longitude,
		EncryptedAtRest: true,
		Favorite:        p.Favorite,
	}, nil
}
