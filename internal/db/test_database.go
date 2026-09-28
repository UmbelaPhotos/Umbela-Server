package db

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

func TestUnlock(dbPath string, masterKey []byte) error {
	hexKey := hex.EncodeToString(masterKey)

	values := url.Values{}
	values.Add("_pragma_key", fmt.Sprintf("x'%s'", hexKey))
	values.Add("_pragma_cipher_page_size", "4096") 

	dsn := fmt.Sprintf("file:%s?%s", dbPath, values.Encode())

	// 2. Abrimos la conexión "cruda"
	sqldb, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("trying to open sqlite3 connection: %w", err)
	}
	defer sqldb.Close()

	// dummy query 
	var version string
	err = sqldb.QueryRow("SELECT sqlite_version();").Scan(&version)

	if err != nil {
		errMsg := err.Error()

		if strings.Contains(errMsg, "file is not a database") {
			return ErrInvalidDatabasePassword
		}
		
		return fmt.Errorf("testing db: %w (%s)", ErrDatabaseCorrupt, errMsg)
	}

	return nil
}