package vault

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/models"
	"umbela-server/internal/storage"
)

const maxPersonNameLength = 100

// minPhotosToListPerson is how many photos a person needs to show up in the
// people list, unless they have a name, which always lists them. Everybody
// else is kept: their faces still appear in each photo and tapping one opens
// all the photos of that person.
const minPhotosToListPerson = 21

func personSummary(r storage.PersonRow) *pb.PersonSummary {
	return &pb.PersonSummary{
		Id:           r.ID,
		Name:         r.Name,
		PhotoCount:   int32(r.PhotoCount),
		CoverPhotoId: r.CoverPhotoID,
	}
}

func photoSummary(p *models.Photo) *pb.PhotoSummary {
	return &pb.PhotoSummary{
		Id:          p.ID,
		TakenAtUnix: p.CapturedAt,
		Blurhash:    p.Blurhash,
		Width:       int32(p.Width),
		Height:      int32(p.Height),
		Favorite:    p.Favorite,
	}
}

// ListPeople returns the people that appear in at least minPhotosToListPerson
// photos or have a name, the most photographed first.
func (v *Vault) ListPeople(ctx context.Context) ([]*pb.PersonSummary, error) {
	rows, err := storage.NewFaceRepository(v.db).ListPeopleWithCounts(ctx, minPhotosToListPerson)
	if err != nil {
		return nil, err
	}
	people := make([]*pb.PersonSummary, 0, len(rows))
	for _, r := range rows {
		people = append(people, personSummary(r))
	}
	return people, nil
}

// LabelPerson gives a person a name. An empty name removes it.
func (v *Vault) LabelPerson(ctx context.Context, personID, name string) (*pb.PersonSummary, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxPersonNameLength {
		return nil, ErrInvalidPersonName
	}

	repo := storage.NewFaceRepository(v.db)
	if err := repo.SetPersonName(ctx, personID, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPersonNotFound
		}
		return nil, err
	}
	row, err := repo.GetPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	return personSummary(*row), nil
}

// GetPersonDetails returns a person with their photos and the albums they appear in.
func (v *Vault) GetPersonDetails(ctx context.Context, personID string) (*pb.GetPersonDetailsResponse, error) {
	repo := storage.NewFaceRepository(v.db)

	row, err := repo.GetPerson(ctx, personID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPersonNotFound
	}
	if err != nil {
		return nil, err
	}

	photos, err := repo.PersonPhotos(ctx, personID)
	if err != nil {
		return nil, err
	}
	albums, err := repo.PersonAlbums(ctx, personID)
	if err != nil {
		return nil, err
	}

	res := &pb.GetPersonDetailsResponse{
		Person: personSummary(*row),
		Photos: make([]*pb.PhotoSummary, 0, len(photos)),
		Albums: make([]*pb.AlbumSummary, 0, len(albums)),
	}
	for _, p := range photos {
		res.Photos = append(res.Photos, photoSummary(p))
	}
	for _, a := range albums {
		res.Albums = append(res.Albums, &pb.AlbumSummary{
			Id:           a.ID,
			Name:         a.Name,
			PhotoCount:   int32(a.PhotoCount),
			CoverPhotoId: a.CoverPhotoID,
		})
	}
	return res, nil
}
