package db

import "errors"

var ErrInvalidDatabasePassword = errors.New("masterkey didnt encrypt database")
var	ErrDatabaseCorrupt         = errors.New("database may be broken")
