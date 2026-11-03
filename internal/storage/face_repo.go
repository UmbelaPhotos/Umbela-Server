package storage

import (
	"context"
	"database/sql"
	"errors"
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
			if _, err := tx.NewInsert().Model(&s.NewPeople).Returning("").Exec(ctx); err != nil {
				return fmt.Errorf("inserting people: %w", err)
			}
		}
		for _, p := range s.UpdatedPeople {
			if _, err := tx.NewUpdate().Model(p).Column("embedding", "samples").WherePK().Exec(ctx); err != nil {
				return fmt.Errorf("updating person %s: %w", p.ID, err)
			}
		}
		if len(s.Links) > 0 {
			if _, err := tx.NewInsert().Model(&s.Links).On("CONFLICT (photo_id, face_id) DO NOTHING").Returning("").Exec(ctx); err != nil {
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

// PersonRow is a person as the people list shows it.
type PersonRow struct {
	ID           string `bun:"id"`
	Name         string `bun:"name"`
	PhotoCount   int    `bun:"photo_count"`
	CoverPhotoID string `bun:"cover_photo_id"`
}

// personSelect is the select list shared by the person queries. The cover is
// the photo where the person's face was detected with the most confidence.
func personSelect(db *bun.DB) *bun.SelectQuery {
	return db.NewSelect().
		TableExpr("faces AS f").
		ColumnExpr("f.id AS id, COALESCE(f.person_name, '') AS name").
		ColumnExpr("(SELECT COUNT(DISTINCT pf.photo_id) FROM photo_faces pf WHERE pf.face_id = f.id) AS photo_count").
		ColumnExpr(`COALESCE((
			SELECT pf.photo_id FROM photo_faces pf WHERE pf.face_id = f.id
			ORDER BY pf.confidence DESC, pf.photo_id DESC LIMIT 1
		), '') AS cover_photo_id`)
}

// ListPeopleWithCounts returns the people that appear in at least minPhotos
// photos, or that have a name and appear in at least one, the most
// photographed first. The rest of the people are still in the database:
// GetPerson and PersonPhotos work for them too.
func (r *FaceRepository) ListPeopleWithCounts(ctx context.Context, minPhotos int) ([]PersonRow, error) {
	const photosOf = "(SELECT COUNT(DISTINCT pf.photo_id) FROM photo_faces pf WHERE pf.face_id = f.id)"

	var rows []PersonRow
	err := personSelect(r.db).
		Where(photosOf+" >= 1").
		Where("("+photosOf+" >= ? OR COALESCE(f.person_name, '') <> '')", max(minPhotos, 1)).
		OrderExpr("photo_count DESC, f.id").
		Scan(ctx, &rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	return rows, nil
}

// GetPerson returns one person, or sql.ErrNoRows when there is none.
func (r *FaceRepository) GetPerson(ctx context.Context, id string) (*PersonRow, error) {
	var row PersonRow
	if err := personSelect(r.db).Where("f.id = ?", id).Scan(ctx, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

// SetPersonName names a person; an empty name removes it. It returns
// sql.ErrNoRows when the person does not exist.
func (r *FaceRepository) SetPersonName(ctx context.Context, id, name string) error {
	res, err := r.db.NewUpdate().
		Model((*models.Face)(nil)).
		Set("person_name = ?", name).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("naming person %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// PersonPhotos returns every photo the person appears in, newest first.
func (r *FaceRepository) PersonPhotos(ctx context.Context, personID string) ([]*models.Photo, error) {
	var photos []*models.Photo
	err := r.db.NewSelect().
		Model(&photos).
		Where("p.id IN (SELECT photo_id FROM photo_faces WHERE face_id = ?)", personID).
		Order("captured_at DESC", "id DESC").
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("listing photos of person %s: %w", personID, err)
	}
	return photos, nil
}

// PersonAlbums returns the albums that hold at least one photo of the person.
func (r *FaceRepository) PersonAlbums(ctx context.Context, personID string) ([]AlbumRow, error) {
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
		Where(`EXISTS (
			SELECT 1 FROM album_photos ap
			JOIN photo_faces pf ON pf.photo_id = ap.photo_id
			WHERE ap.album_id = a.id AND pf.face_id = ?
		)`, personID).
		OrderExpr("a.updated_at DESC, a.id DESC").
		Scan(ctx, &rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("listing albums of person %s: %w", personID, err)
	}
	return rows, nil
}

// PhotoFaceRow is a face found in a photo, with the person it was given to.
type PhotoFaceRow struct {
	PersonID   string  `bun:"person_id"`
	Name       string  `bun:"name"`
	Confidence float64 `bun:"confidence"`
}

// FacesOfPhoto returns the faces found in a photo, from left to right.
func (r *FaceRepository) FacesOfPhoto(ctx context.Context, photoID string) ([]PhotoFaceRow, error) {
	var rows []PhotoFaceRow
	err := r.db.NewSelect().
		TableExpr("photo_faces AS pf").
		Join("JOIN faces AS f ON f.id = pf.face_id").
		ColumnExpr("pf.face_id AS person_id, COALESCE(f.person_name, '') AS name, COALESCE(pf.confidence, 0) AS confidence").
		Where("pf.photo_id = ?", photoID).
		OrderExpr("pf.bbox_min_x").
		Scan(ctx, &rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("listing faces of photo %s: %w", photoID, err)
	}
	return rows, nil
}
