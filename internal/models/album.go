package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Album representa una colección de fotos agrupadas bajo un nombre.
// Puede representar un álbum de Google Takeout, una carpeta, o una agrupación manual.
type Album struct {
	bun.BaseModel `bun:"table:albums,alias:a"`

	ID          int64     `bun:"id,pk,autoincrement"`
	Name        string    `bun:"name,notnull"`
	Description string    `bun:"description"`
	CoverPhotoID int64    `bun:"cover_photo_id"` // FK opcional a photos.id
	CreatedAt   time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt   time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	// Relaciones (pobladas on-demand, no se persisten directamente)
	Photos []*Photo `bun:"m2m:album_photos,join:Album=Photo"`
}

// AlbumPhoto es la tabla pivote many-to-many entre Albums y Photos.
type AlbumPhoto struct {
	bun.BaseModel `bun:"table:album_photos"`

	AlbumID int64  `bun:"album_id,pk"`
	PhotoID int64  `bun:"photo_id,pk"`
	Album   *Album `bun:"rel:belongs-to,join:album_id=id"`
	Photo   *Photo `bun:"rel:belongs-to,join:photo_id=id"`
}
