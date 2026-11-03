package vault

import (
	"context"
	"errors"
	"strings"
	"testing"

	"umbela-server/internal/models"
	"umbela-server/internal/storage"
)

// Ana is in p1..p5 (the best face is in p2), Luis only in p2 and Eva only in
// p6. Only Ana reaches the photos needed to be listed.
func seededPeople(t *testing.T) *Vault {
	t.Helper()
	v := seededVault(t, []models.Photo{
		{ID: "p1", Hash: "h1", FilePath: "x", CapturedAt: 100},
		{ID: "p2", Hash: "h2", FilePath: "x", CapturedAt: 200},
		{ID: "p3", Hash: "h3", FilePath: "x", CapturedAt: 300},
		{ID: "p4", Hash: "h4", FilePath: "x", CapturedAt: 400},
		{ID: "p5", Hash: "h5", FilePath: "x", CapturedAt: 500},
		{ID: "p6", Hash: "h6", FilePath: "x", CapturedAt: 600},
	})
	ctx := context.Background()

	people := []models.Face{
		{ID: "ana", Embedding: []byte{1}, Samples: 5},
		{ID: "luis", Embedding: []byte{1}, Samples: 1},
		{ID: "eva", Embedding: []byte{1}, Samples: 1},
		{ID: "nobody", Embedding: []byte{1}}, // in no photo
	}
	if _, err := v.db.NewInsert().Model(&people).Returning("").Exec(ctx); err != nil {
		t.Fatal(err)
	}

	confidence := map[string]float64{"p1": 0.91, "p2": 0.99, "p3": 0.92, "p4": 0.93, "p5": 0.94}
	var links []models.PhotoFace
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		links = append(links, models.PhotoFace{PhotoID: id, FaceID: "ana", BBoxMinX: 0.1, BBoxMaxX: 0.2, BBoxMaxY: 0.2, Confidence: confidence[id]})
	}
	links = append(links,
		models.PhotoFace{PhotoID: "p2", FaceID: "luis", BBoxMinX: 0.6, BBoxMaxX: 0.7, BBoxMaxY: 0.2, Confidence: 0.95},
		models.PhotoFace{PhotoID: "p6", FaceID: "eva", BBoxMinX: 0.1, BBoxMaxX: 0.2, BBoxMaxY: 0.2, Confidence: 0.93},
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
	if people[0].PhotoCount != 5 || people[0].CoverPhotoId != "p2" {
		t.Fatalf("Ana should have 5 photos and the cover of her best face (p2): %v", people[0])
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
	if got.Name != "Ana García" || got.PhotoCount != 5 {
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
	if res.Person.Id != "ana" || len(res.Photos) != 5 || res.Photos[0].Id != "p5" || res.Photos[4].Id != "p1" {
		t.Fatalf("Ana's 5 photos should come newest first, p5 to p1: %v", res.Photos)
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

	if err := storage.NewPhotoRepository(v.db).Delete(ctx, "p5"); err != nil {
		t.Fatal(err)
	}
	// Ana falls to 4 photos, below the minimum, so she is no longer listed.
	people, err := v.ListPeople(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 0 {
		t.Fatalf("Ana has 4 photos now and should not be listed: %v", people)
	}
	ana, err := v.GetPersonDetails(ctx, "ana")
	if err != nil {
		t.Fatal(err)
	}
	if len(ana.Photos) != 4 {
		t.Fatalf("Ana should keep her 4 remaining photos: %v", ana.Photos)
	}
}
