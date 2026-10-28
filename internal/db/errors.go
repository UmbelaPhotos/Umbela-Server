package db

import "errors"

var ErrInvalidDatabasePassword = errors.New("KEK didnt encrypt database")
var ErrDatabaseCorrupt = errors.New("database may be broken")
