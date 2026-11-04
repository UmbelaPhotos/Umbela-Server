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

// ErrPhotoNotInAlbum is returned when an album cover is not a photo of the album.
var ErrPhotoNotInAlbum = errors.New("photo is not in the album")

// ErrPhotoNotOfPerson is returned when a person's cover is a photo they are not in.
var ErrPhotoNotOfPerson = errors.New("person is not in the photo")
