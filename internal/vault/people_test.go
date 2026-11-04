package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"umbela-server/internal/models"
	"umbela-server/internal/storage"
)

// Ana is in as many photos as the list needs (p1..pN, the best face is in
// p2), Luis only in p2 and Eva only in pe. Only Ana reaches the photos needed
// to be listed.
const anaPhotos = minPhotosToListPerson

func lastAnaPhoto() string { return fmt.Sprintf("p%d", anaPhotos) }

func seededPeople(t *testing.T) *Vault {
	t.Helper()
	var photos []models.Photo
	for i := 1; i <= anaPhotos; i++ {
		photos = append(photos, models.Photo{ID: fmt.Sprintf("p%d", i), Hash: fmt.Sprintf("h%d", i), FilePath: "x", CapturedAt: int64(i) * 100})
	}
	photos = append(photos, models.Photo{ID: "pe", Hash: "he", FilePath: "x", CapturedAt: int64(anaPhotos+1) * 100})
	v := seededVault(t, photos)
	ctx := context.Background()

	people := []models.Face{
		{ID: "ana", Embedding: []byte{1}, Samples: anaPhotos},
		{ID: "luis", Embedding: []byte{1}, Samples: 1},
		{ID: "eva", Embedding: []byte{1}, Samples: 1},
		{ID: "nobody", Embedding: []byte{1}}, // in no photo
	}
	if _, err := v.db.NewInsert().Model(&people).Returning("").Exec(ctx); err != nil {
		t.Fatal(err)
	}

	var links []models.PhotoFace
	for i := 1; i <= anaPhotos; i++ {
		id := fmt.Sprintf("p%d", i)
		conf := 0.91
		if id == "p2" {
			conf = 0.99 // her best face
		}
		links = append(links, models.PhotoFace{PhotoID: id, FaceID: "ana", BBoxMinX: 0.1, BBoxMaxX: 0.2, BBoxMaxY: 0.2, Confidence: conf})
	}
	links = append(links,
		models.PhotoFace{PhotoID: "p2", FaceID: "luis", BBoxMinX: 0.6, BBoxMaxX: 0.7, BBoxMaxY: 0.2, Confidence: 0.95},
		models.PhotoFace{PhotoID: "pe", FaceID: "eva", BBoxMinX: 0.1, BBoxMaxX: 0.2, BBoxMaxY: 0.2, Confidence: 0.93},
	)
	if _, err := v.db.NewInsert().Model(&links).Returning("").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestListPeopleOnlyShowsThoseWithEnoughPhotos(t *testing.T) {
	v := seededPeople(t)
	people, err := v.ListPeople(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 || people[0].Id != "ana" {
		t.Fatalf("only Ana has %d photos or more, got %v", minPhotosToListPerson, people)
	}
	if people[0].PhotoCount != anaPhotos || people[0].CoverPhotoId != "p2" {
		t.Fatalf("Ana should have all her photos and the cover of her best face (p2): %v", people[0])
	}
}

// A person below the minimum is not listed, but is not lost either: the face
// shows in the photo and tapping it opens all of that person's photos.
func TestPeopleBelowTheMinimumAreStillReachable(t *testing.T) {
	v := seededPeople(t)
	ctx := context.Background()

	details, err := v.GetPhotoDetails(ctx, "p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Faces) != 2 || details.Faces[1].PersonId != "luis" {
		t.Fatalf("the photo must show Luis even though he is not listed: %v", details.Faces)
	}

	luis, err := v.GetPersonDetails(ctx, details.Faces[1].PersonId)
	if err != nil {
		t.Fatal(err)
	}
	if luis.Person.PhotoCount != 1 || len(luis.Photos) != 1 || luis.Photos[0].Id != "p2" {
		t.Fatalf("tapping Luis should open his photos: %v", luis)
	}

	// Naming him puts him in the list; removing the name takes him out again.
	if _, err := v.LabelPerson(ctx, "luis", "Luis"); err != nil {
		t.Fatalf("an unlisted person can still be named: %v", err)
	}
	listed, err := v.ListPeople(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[1].Id != "luis" || listed[1].Name != "Luis" {
		t.Fatalf("a named person is always listed, after Ana who has more photos: %v", listed)
	}
	if _, err := v.LabelPerson(ctx, "luis", ""); err != nil {
		t.Fatal(err)
	}
	if listed, _ = v.ListPeople(ctx); len(listed) != 1 {
		t.Fatalf("without a name Luis is hidden again: %v", listed)
	}

	// A named person that appears in no photo is not listed.
	if _, err := v.LabelPerson(ctx, "nobody", "Nadie"); err != nil {
		t.Fatal(err)
	}
	if listed, _ = v.ListPeople(ctx); len(listed) != 1 {
		t.Fatalf("a person without photos must not be listed even with a name: %v", listed)
	}
}

func TestLabelPerson(t *testing.T) {
	v := seededPeople(t)
	ctx := context.Background()

	got, err := v.LabelPerson(ctx, "ana", "  Ana García  ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ana García" || got.PhotoCount != anaPhotos {
		t.Fatalf("unexpected person after naming: %v", got)
	}

	// The name shows up in the list and in the photo's faces.
	people, _ := v.ListPeople(ctx)
	if len(people) != 1 || people[0].Name != "Ana García" {
		t.Fatalf("list does not show the name: %v", people[0])
	}
	details, err := v.GetPhotoDetails(ctx, "p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Faces) != 2 || details.Faces[0].PersonId != "ana" || details.Faces[0].Name != "Ana García" ||
		details.Faces[1].PersonId != "luis" {
		t.Fatalf("faces of p2 should be Ana then Luis, left to right: %v", details.Faces)
	}

	if _, err := v.LabelPerson(ctx, "ana", ""); err != nil {
		t.Fatalf("an empty name removes it: %v", err)
	}
	if _, err := v.LabelPerson(ctx, "nope", "X"); !errors.Is(err, ErrPersonNotFound) {
		t.Fatalf("missing person: got %v", err)
	}
	if _, err := v.LabelPerson(ctx, "ana", strings.Repeat("a", 101)); !errors.Is(err, ErrInvalidPersonName) {
		t.Fatalf("long name: got %v", err)
	}
}

func TestGetPersonDetails(t *testing.T) {
	v := seededPeople(t)
	ctx := context.Background()

	album, err := v.CreateAlbum(ctx, "Viaje")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"p1", "p3"} {
		if err := v.AddPhotoToAlbum(ctx, id, album.Id); err != nil {
			t.Fatal(err)
		}
	}

	res, err := v.GetPersonDetails(ctx, "ana")
	if err != nil {
		t.Fatal(err)
	}
	if res.Person.Id != "ana" || len(res.Photos) != anaPhotos || res.Photos[0].Id != lastAnaPhoto() || res.Photos[anaPhotos-1].Id != "p1" {
		t.Fatalf("Ana's photos should come newest first, last to p1: %v", res.Photos)
	}
	// p1 is in the album, so the album shows up for Ana but not for Luis.
	if len(res.Albums) != 1 || res.Albums[0].Id != album.Id || res.Albums[0].PhotoCount != 2 {
		t.Fatalf("unexpected albums for Ana: %v", res.Albums)
	}
	luis, err := v.GetPersonDetails(ctx, "luis")
	if err != nil {
		t.Fatal(err)
	}
	if len(luis.Albums) != 0 || len(luis.Photos) != 1 {
		t.Fatalf("Luis is only in p2, which is in no album: %v", luis)
	}

	if _, err := v.GetPersonDetails(ctx, "nope"); !errors.Is(err, ErrPersonNotFound) {
		t.Fatalf("missing person: got %v", err)
	}
}

func TestDeletingAPhotoRemovesItsFaces(t *testing.T) {
	v := seededPeople(t)
	ctx := context.Background()

	if err := storage.NewPhotoRepository(v.db).Delete(ctx, lastAnaPhoto()); err != nil {
		t.Fatal(err)
	}
	// Ana falls one photo below, below the minimum, so she is no longer listed.
	people, err := v.ListPeople(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 0 {
		t.Fatalf("Ana is one photo short now and should not be listed: %v", people)
	}
	ana, err := v.GetPersonDetails(ctx, "ana")
	if err != nil {
		t.Fatal(err)
	}
	if len(ana.Photos) != anaPhotos-1 {
		t.Fatalf("Ana should keep her remaining photos: %v", ana.Photos)
	}
}
