package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Photo struct {
	bun.BaseModel `bun:"table:photos,alias:p"`

	ID          string    `bun:"id,pk,type:text"`
	Hash        string    `bun:"hash,unique,not null"` // Para deduplicación
	Title       string    `bun:"title"`
	Description string    `bun:"description"`
	Artist      string    `bun:"artist"`
	CapturedAt  int64     `bun:"captured_at,index"` // Unix timestamp para velocidad
	Latitude    float64   `bun:"latitude"`
	Longitude   float64   `bun:"longitude"`
	FilePath    string    `bun:"file_path,not null"`
	ThumbPath   string    `bun:"thumb_path"`
	Blurhash    string    `bun:"blurhash"`
	CreatedAt   time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`

	Faces []Face `bun:"m2m:photo_faces,join:Photo=Face"`
}

type Face struct {
	bun.BaseModel `bun:"table:faces,alias:f"`

	ID         string `bun:"id,pk,type:text"`
	PersonName string `bun:"person_name"`
	Embedding  []byte `bun:"embedding,notnull"`

	Photos []Photo `bun:"m2m:photo_faces,join:Face=Photo"`
}

type PhotoFace struct {
	bun.BaseModel `bun:"table:photo_faces,alias:pf"`

	PhotoID		string `bun:"photo_id,pk,type:text`
	FaceID		string `bun:"face_id,pk,type:text`

	Photo		*Photo `bun:"rel:belongs-to,join:photo_id=id"`
	Face		*Face  `bun:"rel:belongs-to,join:face_id=id"`

	BBoxMinX	float64	`bun:"bbox_min_x,notnull"`
	BBoxMinY	float64	`bun:"bbox_min_y,notnull"`
	BBoxMaxX	float64	`bun:"bbox_max_x,notnull"`
	BBoxMaxY	float64	`bun:"bbox_min_y,notnull"`

	Confidence 	float64	`bun:"confidence"`
}
