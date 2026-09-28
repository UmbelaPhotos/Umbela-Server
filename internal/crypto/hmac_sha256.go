package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func HashText(plaintext string, salt []byte) string{
	mac:=hmac.New(sha256.New, salt)

	mac.Write([]byte(plaintext))

	hashBytes:=mac.Sum(nil)

	return hex.EncodeToString(hashBytes)
}