package models

import (
	"github.com/uptrace/bun"
	"time"
)

type Photo struct {
	bun.BaseModel `bun:"table:photos,alias:p"`

	ID          int64     `bun:"id,pk,autoincrement"`
	Hash        string    `bun:"hash,unique,not null"` // Para deduplicación
	Title       string    `bun:"title"`
	Description string    `bun:"description"`
	Artist 		string	  `bun:"artist"`
	CapturedAt  int64     `bun:"captured_at,index"` // Unix timestamp para velocidad
	Latitude    float64   `bun:"latitude"`
	Longitude   float64   `bun:"longitude"`
	FilePath    string    `bun:"file_path,not null"`
	ThumbPath   string    `bun:"thumb_path"`
	Blurhash    string    `bun:"blurhash"`
	CreatedAt   time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
