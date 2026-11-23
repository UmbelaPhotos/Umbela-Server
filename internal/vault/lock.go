package vault

import (
	"context"
	"fmt"
	"time"

	"umbela-server/internal/config"
	"umbela-server/internal/crypto"
	"umbela-server/internal/db"
)

// Acquire marks the start of a request that uses the vault. While the returned
// release function has not been called, Lock waits, so the database is never
// closed in the middle of a query or a stream. It returns ErrLocked if the
// vault is locked.
func (v *Vault) Acquire() (release func(), err error) {
	v.mu.RLock()
	if v.db == nil {
		v.mu.RUnlock()
		return nil, ErrLocked
	}
	return v.mu.RUnlock, nil
}

// Lock stops the face scanner, waits for the requests in progress, flushes and
// closes the database and zeroes the DEK in memory. The image processor stays
// up (libvips cannot be restarted reliably). Unlock brings the vault back.
func (v *Vault) Lock() error {
	v.StopFaceScanner()

	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db == nil {
		return nil
	}
	err := db.Close(v.db)
	v.db = nil
	crypto.CleanBytes(v.config.DEK)
	v.config.DEK = nil
	return err
}

// Unlock checks the KEK by decrypting the config file with it, which also gives
// back the DEK, and reopens the database and the face scanner if the vault was
// locked. It returns ErrInvalidPassword when the KEK is wrong.
func (v *Vault) Unlock(kek []byte) error {
	cfg, err := config.Read(v.path, kek)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPassword, err)
	}
	dek := cfg.Vault.DEK

	v.mu.Lock()
	if v.db != nil { // already open: the KEK was valid, nothing else to do
		v.mu.Unlock()
		crypto.CleanBytes(dek)
		return nil
	}
	database, err := db.InitDB(v.path, dek)
	if err != nil {
		v.mu.Unlock()
		crypto.CleanBytes(dek)
		return err
	}
	v.db = database
	v.config.DEK = dek
	v.mu.Unlock()

	v.scanMu.Lock()
	v.startScannerLocked()
	v.scanMu.Unlock()
	return nil
}

// StartFaceScanner runs RunFaceScanner in the background until parent is
// cancelled or the vault is locked. Unlock starts it again.
func (v *Vault) StartFaceScanner(parent context.Context) {
	v.scanMu.Lock()
	defer v.scanMu.Unlock()
	v.scanParent = parent
	v.startScannerLocked()
}

func (v *Vault) startScannerLocked() {
	if v.scanDone != nil || v.scanParent == nil {
		return
	}
	ctx, cancel := context.WithCancel(v.scanParent)
	done := make(chan struct{})
	v.scanCancel, v.scanDone = cancel, done

	go func() {
		defer close(done)
		if err := v.RunFaceScanner(ctx); err != nil && ctx.Err() == nil {
			fmt.Printf("Face recognition is off: %v\n", err)
		}
	}()
}

// StopFaceScanner stops the scanner and waits up to 10 seconds for it.
func (v *Vault) StopFaceScanner() {
	v.scanMu.Lock()
	cancel, done := v.scanCancel, v.scanDone
	v.scanCancel, v.scanDone = nil, nil
	v.scanMu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}
