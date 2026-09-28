package vault

import "errors"

// ErrUserNotFound es un error tipado y único en memoria
var ErrUserNotFound = errors.New("user doesn't exists")

// ErrInvalidPassword se usaría si la base de datos rechaza la MasterKey
var ErrInvalidPassword = errors.New("wrong password")