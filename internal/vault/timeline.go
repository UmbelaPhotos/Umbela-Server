package vault

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/models"
)

const (
	defaultTimelinePage = 300
	maxTimelinePage     = 1000

	// Range strftime handles (years 0001..9999); anything else is junk data.
	minValidCapturedAt = 1
	maxValidCapturedAt = 253402300799
)

// encodeCursor / decodeCursor: opaque "capturedAt:id" so the client never
// depends on the shape.
func encodeCursor(capturedAt int64, id string) string {
	raw := strconv.FormatInt(capturedAt, 10) + ":" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (int64, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", ErrInvalidCursor
	}
	ts, id, ok := strings.Cut(string(raw), ":")
	if !ok || id == "" {
		return 0, "", ErrInvalidCursor
	}
	capturedAt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return 0, "", ErrInvalidCursor
	}
	return capturedAt, id, nil
}

// GetTimelinePage returns one page of photos, newest first, inside the
// optional [fromUnix, toUnix] range (0 means open). Keyset pagination on
// (captured_at, id): stable while photos are added or removed, and the cost
// of a page does not grow with its depth. nextCursor is empty on the last page.
func (v *Vault) GetTimelinePage(
	ctx context.Context,
	fromUnix, toUnix int64,
	pageSize int,
	cursor string,
) (photos []*pb.PhotoSummary, nextCursor string, err error) {
	if pageSize <= 0 {
		pageSize = defaultTimelinePage
	}
	if pageSize > maxTimelinePage {
		pageSize = maxTimelinePage
	}

	var rows []models.Photo
	query := v.db.NewSelect().
		Model(&rows).
		Order("captured_at DESC", "id DESC").
		// One extra row tells us whether another page exists.
		Limit(int64(pageSize) + 1)

	if fromUnix > 0 {
		query = query.Where("captured_at >= ?", fromUnix)
	}
	if toUnix > 0 {
		query = query.Where("captured_at <= ?", toUnix)
	}
	if cursor != "" {
		ts, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query = query.Where("(captured_at < ? OR (captured_at = ? AND id < ?))", ts, ts, id)
	}

	if err := query.Scan(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, "", err
	}

	if len(rows) > pageSize {
		rows = rows[:pageSize]
		last := rows[len(rows)-1]
		nextCursor = encodeCursor(last.CapturedAt, last.ID)
	}

	photos = make([]*pb.PhotoSummary, 0, len(rows))
	for _, p := range rows {
		photos = append(photos, &pb.PhotoSummary{
			Id:          p.ID,
			TakenAtUnix: p.CapturedAt,
			IsVideo:     false,
			Blurhash:    p.Blurhash,
			Width:       int32(p.Width),
			Height:      int32(p.Height),
			Favorite:    p.Favorite,
		})
	}
	return photos, nextCursor, nil
}

type bucketRow struct {
	Year  int `bun:"year"`
	Month int `bun:"month"`
	Count int `bun:"count"`
}

// GetTimelineBuckets counts photos per month (in the viewer's UTC offset),
// newest month first. Each bucket carries the inclusive unix range that
// GetTimelinePage accepts, so the client can load exactly that month.
func (v *Vault) GetTimelineBuckets(ctx context.Context, tzOffsetSeconds int) ([]*pb.TimelineBucket, error) {
	var rows []bucketRow
	err := v.db.NewSelect().
		TableExpr("photos").
		ColumnExpr("CAST(strftime('%Y', captured_at + ?, 'unixepoch') AS INTEGER) AS year", tzOffsetSeconds).
		ColumnExpr("CAST(strftime('%m', captured_at + ?, 'unixepoch') AS INTEGER) AS month", tzOffsetSeconds).
		ColumnExpr("COUNT(*) AS count").
		Where("captured_at BETWEEN ? AND ?", minValidCapturedAt, maxValidCapturedAt).
		GroupExpr("year, month").
		OrderExpr("year DESC, month DESC").
		Scan(ctx, &rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	loc := time.FixedZone("viewer", tzOffsetSeconds)
	buckets := make([]*pb.TimelineBucket, 0, len(rows))
	for _, r := range rows {
		if r.Year <= 0 || r.Month < 1 || r.Month > 12 {
			continue
		}
		start := time.Date(r.Year, time.Month(r.Month), 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, 0)
		buckets = append(buckets, &pb.TimelineBucket{
			Year:     int32(r.Year),
			Month:    int32(r.Month),
			Count:    int32(r.Count),
			FromUnix: start.Unix(),
			ToUnix:   end.Unix() - 1,
		})
	}
	return buckets, nil
}
