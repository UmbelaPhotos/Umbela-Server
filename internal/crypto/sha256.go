package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func HashText(plaintext string, salt []byte) string{
	mac:=hmac.New(sha256.New, salt)

	mac.Write([]byte(plaintext))

	hashBytes:=mac.Sum(nil)

	return hex.EncodeToString(hashBytes)
}
func ComputeSHA256(filePath string) (string, error) {
	openedFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening file %w", err)
	}
	defer openedFile.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, openedFile)
	if err != nil {
		return "", fmt.Errorf("Error hashing the file: %w", err)
	}
	hashString := fmt.Sprintf("%x", hasher.Sum(nil))
	return hashString, nil
}