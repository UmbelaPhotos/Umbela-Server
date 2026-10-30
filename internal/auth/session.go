package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session),
	}
	// Limpieza periódica automática de tokens expirados
	go store.startCleaner(10 * time.Minute)
	return store
}

// Create genera un token aleatorio de 32 bytes y registra la sesión en RAM
func (s *SessionStore) Create(ttl time.Duration) (string, time.Time, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", time.Time{}, err
	}

	token := hex.EncodeToString(bytes)
	now := time.Now()
	expiresAt := now.Add(ttl)

	s.mu.Lock()
	s.sessions[token] = &Session{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	s.mu.Unlock()

	return token, expiresAt, nil
}

// IsValid verifica si el token existe en RAM y si aún no ha caducado
func (s *SessionStore) IsValid(token string) bool {
	if token == "" {
		return false
	}

	s.mu.RLock()
	session, exists := s.sessions[token]
	s.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(session.ExpiresAt) {
		s.Revoke(token)
		return false
	}

	return true
}

// Revoke elimina un token específico de la memoria
func (s *SessionStore) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// DestroyAll purga inmediatamente todas las sesiones activas en RAM (Usado en Lock RPC)
func (s *SessionStore) DestroyAll() {
	s.mu.Lock()
	s.sessions = make(map[string]*Session)
	s.mu.Unlock()
}

// HasActiveSessions devuelve true si hay al menos una sesión válida descorriendo
func (s *SessionStore) HasActiveSessions() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, sess := range s.sessions {
		if now.Before(sess.ExpiresAt) {
			return true
		}
	}
	return false
}

// startCleaner elimina en segundo plano las sesiones viejas que ya caducaron
func (s *SessionStore) startCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for token, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}