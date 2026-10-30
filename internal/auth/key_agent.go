package auth

import (
	"errors"
	"sync"
)

var ErrVaultLocked = errors.New("el vault está bloqueado y la clave no está en RAM")

type KeyAgent struct {
	mu        sync.RWMutex
	masterKey []byte
	isUnlocked bool
}

func NewKeyAgent() *KeyAgent {
	return &KeyAgent{
		isUnlocked: false,
	}
}

// SetKey guarda una copia de la KEK / Master Key en RAM cuando se desbloquea el Vault
func (k *KeyAgent) SetKey(key []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Si ya había una clave antigua, borrarla
	k.wipeKeyLocked()

	// Copiar bytes para evitar que el caller modifique la memoria externa
	k.masterKey = make([]byte, len(key))
	copy(k.masterKey, key)
	k.isUnlocked = true
}

// GetKey entrega una copia de la clave en RAM a los servicios de cifrado/descifrado
func (k *KeyAgent) GetKey() ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.isUnlocked || len(k.masterKey) == 0 {
		return nil, ErrVaultLocked
	}

	keyCopy := make([]byte, len(k.masterKey))
	copy(keyCopy, k.masterKey)
	return keyCopy, nil
}

// ClearKey destruye físicamente la clave en RAM sobrescribiendo los bytes con ceros
func (k *KeyAgent) ClearKey() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.wipeKeyLocked()
}

// IsUnlocked informa si la clave está disponible en memoria
func (k *KeyAgent) IsUnlocked() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()

	return k.isUnlocked && len(k.masterKey) > 0
}

// wipeKeyLocked realiza la limpieza con ceros (debe llamarse con el mutex en Lock)
func (k *KeyAgent) wipeKeyLocked() {
	if len(k.masterKey) > 0 {
		for i := range k.masterKey {
			k.masterKey[i] = 0 // Sobrescribe cada byte en memoria
		}
		k.masterKey = nil
	}
	k.isUnlocked = false
}