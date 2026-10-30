package vault

import (
	"context"
	"fmt"
	"testing"
	"time"

	"umbela-server/internal/db"
	"umbela-server/internal/models"
)

func seededVault(t *testing.T, photos []models.Photo) *Vault {
	t.Helper()
	database, err := db.InitDB(t.TempDir(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if _, err := database.NewInsert().Model(&photos).Returning("").Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &Vault{db: database}
}

// 2500 photos over ~5 years, many sharing the same second.
func manyPhotos() []models.Photo {
	base := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Unix()
	out := make([]models.Photo, 2500)
	for i := range out {
		out[i] = models.Photo{
			ID:         fmt.Sprintf("id-%05d", i),
			Hash:       fmt.Sprintf("h%d", i),
			FilePath:   "x",
			CapturedAt: base + int64(i/5)*3600*17, // 5 photos per timestamp
			Blurhash:   "LEHV6nWB2yk8pyo0adR*.7kCMdnj",
		}
	}
	return out
}

func TestTimelinePagesNeverSkipOrRepeat(t *testing.T) {
	v := seededVault(t, manyPhotos())
	ctx := context.Background()

	seen := map[string]bool{}
	var last int64 = 1 << 62
	cursor, pages := "", 0
	for {
		page, next, err := v.GetTimelinePage(ctx, 0, 0, 337, cursor)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, p := range page {
			if seen[p.Id] {
				t.Fatalf("repeated %s", p.Id)
			}
			seen[p.Id] = true
			if p.TakenAtUnix > last {
				t.Fatalf("out of order at %s", p.Id)
			}
			last = p.TakenAtUnix
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != 2500 || pages != 8 {
		t.Fatalf("got %d photos in %d pages", len(seen), pages)
	}
}

func TestTimelinePageSizeDefaultsAndCaps(t *testing.T) {
	v := seededVault(t, manyPhotos())
	ctx := context.Background()
	p, next, _ := v.GetTimelinePage(ctx, 0, 0, 0, "")
	if len(p) != defaultTimelinePage || next == "" {
		t.Fatalf("default page: %d", len(p))
	}
	p, _, _ = v.GetTimelinePage(ctx, 0, 0, 999999, "")
	if len(p) != maxTimelinePage {
		t.Fatalf("capped page: %d", len(p))
	}
	if _, _, err := v.GetTimelinePage(ctx, 0, 0, 10, "garbage!"); err != ErrInvalidCursor {
		t.Fatalf("want ErrInvalidCursor, got %v", err)
	}
}

func TestBucketsMatchTheirOwnRanges(t *testing.T) {
	v := seededVault(t, manyPhotos())
	ctx := context.Background()

	for _, tz := range []int{0, -6 * 3600, 5*3600 + 1800} {
		buckets, err := v.GetTimelineBuckets(ctx, tz)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for i, b := range buckets {
			total += int(b.Count)
			if i > 0 {
				prev := buckets[i-1]
				if prev.Year*12+prev.Month <= b.Year*12+b.Month {
					t.Fatalf("not newest first at %d/%d", b.Year, b.Month)
				}
			}
			// Loading the bucket's range returns exactly its count.
			n, cursor := 0, ""
			for {
				page, next, err := v.GetTimelinePage(ctx, b.FromUnix, b.ToUnix, 0, cursor)
				if err != nil {
					t.Fatal(err)
				}
				n += len(page)
				if next == "" {
					break
				}
				cursor = next
			}
			if n != int(b.Count) {
				t.Fatalf("tz %d bucket %d-%02d: count %d but range has %d", tz, b.Year, b.Month, b.Count, n)
			}
		}
		if total != 2500 {
			t.Fatalf("tz %d: buckets add up to %d", tz, total)
		}
	}
}

func TestBucketsRespectViewerTimezone(t *testing.T) {
	// 2024-03-01 02:30 UTC is still February 29th in UTC-6.
	ts := time.Date(2024, 3, 1, 2, 30, 0, 0, time.UTC).Unix()
	v := seededVault(t, []models.Photo{{ID: "a", Hash: "a", FilePath: "x", CapturedAt: ts}})
	utc, _ := v.GetTimelineBuckets(context.Background(), 0)
	cdmx, _ := v.GetTimelineBuckets(context.Background(), -6*3600)
	if utc[0].Month != 3 || cdmx[0].Month != 2 {
		t.Fatalf("utc month %d, local month %d", utc[0].Month, cdmx[0].Month)
	}
}

func TestBucketsOnEmptyLibrary(t *testing.T) {
	v := seededVault(t, []models.Photo{{ID: "a", Hash: "a", FilePath: "x", CapturedAt: 0}})
	b, err := v.GetTimelineBuckets(context.Background(), 0)
	if err != nil || len(b) != 0 {
		t.Fatalf("got %v, %v", b, err)
	}
}
