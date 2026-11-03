package vault

import "errors"

// ErrUserNotFound es un error tipado y único en memoria
var ErrUserNotFound = errors.New("user doesn't exists")

// ErrInvalidPassword se usaría si la base de datos rechaza la KEK
var ErrInvalidPassword = errors.New("wrong password")

// ErrInvalidCursor is returned when a timeline cursor was not issued by us.
var ErrInvalidCursor = errors.New("invalid timeline cursor")

// ErrPhotoNotFound is returned when no photo has the requested id.
var ErrPhotoNotFound = errors.New("photo not found")

// ErrAlbumNotFound is returned when no album has the requested id.
var ErrAlbumNotFound = errors.New("album not found")

// ErrInvalidAlbumName is returned when an album would be created without a name.
var ErrInvalidAlbumName = errors.New("invalid album name")

// ErrPersonNotFound is returned when no person has the requested id.
var ErrPersonNotFound = errors.New("person not found")

// ErrInvalidPersonName is returned when a person name is too long.
var ErrInvalidPersonName = errors.New("invalid person name")
