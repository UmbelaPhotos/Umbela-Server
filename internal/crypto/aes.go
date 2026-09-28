package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func EncryptGCM(key []byte, dataBytes []byte) ([]byte, error){
	block, err:=aes.NewCipher(key)
	if err!=nil{
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err!=nil{
		return nil, fmt.Errorf("starting gcm: %w", err)
	}

	// Number used once
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil{
		return nil, err
	}

	cipherBytes := gcm.Seal(nonce, nonce, dataBytes, nil) 
	
	return cipherBytes, nil
}

func DecryptGCM(key []byte, encriptedBytes []byte) ([]byte, error){
	block, err:=aes.NewCipher(key)
	if err!=nil{
		return nil,fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err!=nil{
		return nil, fmt.Errorf("starting gcm: %w", err)
	}

	// Get nonce
	nonceSize:=gcm.NonceSize()
	nonce, actualCipherData := encriptedBytes[:nonceSize], encriptedBytes[nonceSize:]

	dataBytes, err := gcm.Open(nil, nonce, actualCipherData, nil)
	if err!=nil{
		return nil, fmt.Errorf("tried desciphering data with masterkey: %w", err)
	}

	return dataBytes, nil
}