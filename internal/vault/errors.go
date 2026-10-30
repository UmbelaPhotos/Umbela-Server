package vault

import "errors"

// ErrUserNotFound es un error tipado y único en memoria
var ErrUserNotFound = errors.New("user doesn't exists")

// ErrInvalidPassword se usaría si la base de datos rechaza la KEK
var ErrInvalidPassword = errors.New("wrong password")

// ErrInvalidCursor is returned when a timeline cursor was not issued by us.
var ErrInvalidCursor = errors.New("invalid timeline cursor")
