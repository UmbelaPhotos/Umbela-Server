package crypto

import (
	"golang.org/x/crypto/argon2"
)

func DeriveKEK(passwordBytes []byte, salt []byte) []byte {
	var time uint32 = 3
	var memory uint32 = 64 * 1024
	var threads uint8 = 4
	var keyLen uint32 = 32

	KEK := argon2.IDKey(passwordBytes, salt, time, memory, threads, keyLen)

	return KEK
}
