package crypto

import (
	"encoding/binary"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

func DeriveKEK(username string, password, salt []byte) []byte {
	const (
		time    uint32 = 3
		memory  uint32 = 64 * 1024
		threads uint8  = 4
		keyLen  uint32 = 32
	)

	user := []byte(cases.Fold().String(norm.NFKC.String(username)))
	pass := norm.NFKC.Bytes(password)

	in := make([]byte, 0, 8+len(user)+len(pass))
	in = binary.BigEndian.AppendUint32(in, uint32(len(user)))
	in = append(in, user...)
	in = binary.BigEndian.AppendUint32(in, uint32(len(pass)))
	in = append(in, pass...)

	kek := argon2.IDKey(in, salt, time, memory, threads, keyLen)

	return kek
}