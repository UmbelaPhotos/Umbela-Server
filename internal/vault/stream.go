package vault

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/crypto"
	"umbela-server/internal/models"
)

const chunkSize = 64 * 1024 

func (v *Vault) StreamThumbnail(ctx context.Context, photoID string, stream *connect.ServerStream[pb.Chunk]) error {
	t0 := time.Now() // [perf]
	var photo models.Photo
	err := v.db.NewSelect().Model(&photo).Where("id = ?", photoID).Scan(ctx)
	if err != nil {
		return err
	}
	dbMs := time.Since(t0).Milliseconds() // [perf]

	return v.streamDecryptedFile("thumb", photoID, dbMs, photo.ThumbPath, stream)
}

func (v *Vault) StreamFullImage(ctx context.Context, photoID string, stream *connect.ServerStream[pb.Chunk]) error {
	var photo models.Photo
	err := v.db.NewSelect().Model(&photo).Where("id = ?", photoID).Scan(ctx)
	if err != nil {
		return err
	}

	return v.streamDecryptedFile("FULL", photoID, 0, photo.FilePath, stream)
}

func (v *Vault) streamDecryptedFile(kind, photoID string, dbMs int64, path string, stream *connect.ServerStream[pb.Chunk]) error {
	t1 := time.Now() // [perf]
	encryptedData, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decryptedData, err := crypto.DecryptGCM(encryptedData, v.config.DEK)
	if err != nil {
		return err
	}
	prepMs := time.Since(t1).Milliseconds() // [perf]
	t2 := time.Now()                        // [perf]
	defer func() {                          // [perf]
		fmt.Printf("[perf] %s %s: %d KB, db=%dms read+decrypt=%dms send=%dms\n",
			kind, photoID[:8], len(decryptedData)/1024, dbMs, prepMs, time.Since(t2).Milliseconds())
	}()

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