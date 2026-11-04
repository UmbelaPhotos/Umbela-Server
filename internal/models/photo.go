package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Photo struct {
	bun.BaseModel `bun:"table:photos,alias:p"`

	ID          string    `bun:"id,pk,type:text"`
	Hash        string    `bun:"hash,unique,notnull"` // Para deduplicación
	Title       string    `bun:"title"`
	Description string    `bun:"description"`
	Artist      string    `bun:"artist"`
	Width       int       `bun:"width"`
	Height      int       `bun:"Height"`
	CapturedAt  int64     `bun:"captured_at"`
	Latitude    float64   `bun:"latitude"`
	Longitude   float64   `bun:"longitude"`
	FilePath    string    `bun:"file_path,notnull"`
	ThumbPath   string    `bun:"thumb_path"`
	Blurhash    string    `bun:"blurhash"`
	CreatedAt   time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	Favorite    bool      `bun:"favorite,default:false"`
	// FacesScannedAt is the unix time when the face scan of this photo
	// finished; 0 means the photo is still waiting to be scanned.
	FacesScannedAt int64 `bun:"faces_scanned_at,notnull,default:0"`

	Faces []*Face `bun:"m2m:photo_faces,join:Photo=Face"`
}

type Face struct {
	bun.BaseModel `bun:"table:faces,alias:f"`

	ID         string `bun:"id,pk,type:text"`
	PersonName string `bun:"person_name"`
	// Embedding is the centroid (normalized mean) of the embeddings of all the
	// faces assigned to this person, and Samples is how many that is.
	CoverPhotoID string `bun:"cover_photo_id,type:text,nullzero"` // FK opcional a photos.id (NULL = sin portada)
	Embedding    []byte `bun:"embedding,notnull"`
	Samples      int    `bun:"samples,notnull,default:0"`

	Photos []*Photo `bun:"m2m:photo_faces,join:Face=Photo"`
}

type PhotoFace struct {
	bun.BaseModel `bun:"table:photo_faces,alias:pf"`

	PhotoID string `bun:"photo_id,pk,type:text"`
	FaceID  string `bun:"face_id,pk,type:text"`

	Photo *Photo `bun:"rel:belongs-to,join:photo_id=id"`
	Face  *Face  `bun:"rel:belongs-to,join:face_id=id"`

	// The box is relative to the (auto-rotated) photo: 0 is the top/left edge
	// and 1 the bottom/right edge.
	BBoxMinX float64 `bun:"bbox_min_x,notnull"`
	BBoxMinY float64 `bun:"bbox_min_y,notnull"`
	BBoxMaxX float64 `bun:"bbox_max_x,notnull"`
	BBoxMaxY float64 `bun:"bbox_max_y,notnull"`

	Confidence float64 `bun:"confidence"`
	// Embedding of this particular face, kept to be able to regroup people.
	Embedding []byte `bun:"embedding"`
}
