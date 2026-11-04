package vault

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/models"
	"umbela-server/internal/storage"
)

// ListAlbums returns every album with its photo count and cover.
func (v *Vault) ListAlbums(ctx context.Context) ([]*pb.AlbumSummary, error) {
	rows, err := storage.NewAlbumRepository(v.db).ListWithCounts(ctx)
	if err != nil {
		return nil, err
	}
	albums := make([]*pb.AlbumSummary, 0, len(rows))
	for _, r := range rows {
		albums = append(albums, &pb.AlbumSummary{
			Id:           r.ID,
			Name:         r.Name,
			PhotoCount:   int32(r.PhotoCount),
			CoverPhotoId: r.CoverPhotoID,
		})
	}
	return albums, nil
}

// CreateAlbum makes an empty album. Albums only group photos; no file moves.
func (v *Vault) CreateAlbum(ctx context.Context, name string) (*pb.AlbumSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidAlbumName
	}
	album := &models.Album{Name: name}
	if err := storage.NewAlbumRepository(v.db).Save(ctx, album); err != nil {
		return nil, err
	}
	return &pb.AlbumSummary{Id: album.ID, Name: album.Name}, nil
}

// AddPhotoToAlbum links an existing photo to an existing album.
func (v *Vault) AddPhotoToAlbum(ctx context.Context, photoID, albumID string) error {
	photos := storage.NewPhotoRepository(v.db)
	albums := storage.NewAlbumRepository(v.db)

	if _, err := albums.GetByID(ctx, albumID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlbumNotFound
		}
		return err
	}
	if _, err := photos.GetByID(ctx, photoID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPhotoNotFound
		}
		return err
	}
	return albums.AddPhoto(ctx, albumID, photoID)
}

// DeletePhoto removes the photo from the database and its encrypted original
// and thumbnail from disk.
func (v *Vault) DeletePhoto(ctx context.Context, photoID string) error {
	photos := storage.NewPhotoRepository(v.db)

	photo, err := photos.GetByID(ctx, photoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPhotoNotFound
		}
		return err
	}
	if err := photos.Delete(ctx, photoID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPhotoNotFound
		}
		return err
	}

	// The row is gone; a file that is already missing is not a failure.
	for _, path := range []string{photo.FilePath, photo.ThumbPath} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// SetAlbumCover picks the photo that represents the album. It has to be in it.
func (v *Vault) SetAlbumCover(ctx context.Context, albumID, photoID string) (*pb.AlbumSummary, error) {
	albums := storage.NewAlbumRepository(v.db)
	if err := albums.SetCover(ctx, albumID, photoID); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrAlbumNotFound
		case errors.Is(err, storage.ErrPhotoNotInAlbum):
			return nil, ErrPhotoNotInAlbum
		}
		return nil, err
	}
	rows, err := albums.ListWithCounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID == albumID {
			return &pb.AlbumSummary{
				Id:           r.ID,
				Name:         r.Name,
				PhotoCount:   int32(r.PhotoCount),
				CoverPhotoId: r.CoverPhotoID,
			}, nil
		}
	}
	return nil, ErrAlbumNotFound
}
