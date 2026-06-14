package models

import (
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID           int64     `bun:"id,pk,autoincrement"`
	Username     string    `bun:"username,notnull,unique"`
	PasswordHash string    `bun:"password_hash,notnull"`
	CreatedAt    time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// Session es un token de sesión generado al hacer login.
// Se almacena en BD para poder invalidarlo con logout.
type Session struct {
	bun.BaseModel `bun:"table:sessions,alias:s"`

	Token     string    `bun:"token,pk"`
	UserID    int64     `bun:"user_id,notnull"`
	Username  string    `bun:"username,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
