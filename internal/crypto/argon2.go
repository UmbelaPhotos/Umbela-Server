package crypto

import (
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

func DeriveMasterKey(passwordBytes []byte, salt []byte) []byte{
	// OWASP/RFC 9106 recommended
	var time uint32 = 1
	var memory uint32 = 64*1024
	var threads uint8 = 4
	var keyLen uint32 =32

	masterKey:=argon2.IDKey(passwordBytes, salt, time, memory, threads, keyLen)

	return masterKey
}

// Unique per user
// Salt will be saved in the first 16 bytes of the config
func GenerateSalt() ([]byte, error){
	salt := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil{
		return nil, fmt.Errorf("making salt: %w", err)
	}

	return salt, nil
}