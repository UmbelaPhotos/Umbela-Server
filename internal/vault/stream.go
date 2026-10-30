package vault

import (
	"context"
	"os"

	"connectrpc.com/connect"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/crypto"
	"umbela-server/internal/models"
)

const chunkSize = 64 * 1024 

func (v *Vault) StreamThumbnail(ctx context.Context, photoID string, stream *connect.ServerStream[pb.Chunk]) error {
	var photo models.Photo
	err := v.db.NewSelect().Model(&photo).Where("id = ?", photoID).Scan(ctx)
	if err != nil {
		return err
	}

	return v.streamDecryptedFile(photo.ThumbPath, stream)
}

func (v *Vault) StreamFullImage(ctx context.Context, photoID string, stream *connect.ServerStream[pb.Chunk]) error {
	var photo models.Photo
	err := v.db.NewSelect().Model(&photo).Where("id = ?", photoID).Scan(ctx)
	if err != nil {
		return err
	}

	return v.streamDecryptedFile(photo.FilePath, stream)
}

func (v *Vault) streamDecryptedFile(path string, stream *connect.ServerStream[pb.Chunk]) error {
	encryptedData, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decryptedData, err := crypto.DecryptGCM(encryptedData, v.config.DEK)
	if err != nil {
		return err
	}

	totalBytes := len(decryptedData)
	for i := 0; i < totalBytes; i += chunkSize {
		end := i + chunkSize
		if end > totalBytes {
			end = totalBytes
		}

		chunk := &pb.Chunk{
			Data: decryptedData[i:end],
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (v *Vault) ToggleFavorite(ctx context.Context, photoID string) (bool, error) {
	var photo models.Photo
	err := v.db.NewSelect().Model(&photo).Where("id = ?", photoID).Scan(ctx)
	if err != nil {
		return false, err
	}

	photo.Favorite = !photo.Favorite

	_, err = v.db.NewUpdate().Model(&photo).Column("favorite").Where("id = ?", photoID).Exec(ctx)
	if err != nil {
		return false, err
	}

	return photo.Favorite, nil
}