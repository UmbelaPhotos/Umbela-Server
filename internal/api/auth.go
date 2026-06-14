package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"allium-server/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
		return
	}
	if len(req.Username) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "el usuario debe tener al menos 3 caracteres"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "la contraseña debe tener al menos 8 caracteres"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno"})
		return
	}

	user := &models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
	}
	if _, err := s.db.NewInsert().Model(user).Exec(r.Context()); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ese nombre de usuario ya existe"})
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
		return
	}

	var user models.User
	err := s.db.NewSelect().Model(&user).
		Where("username = ?", req.Username).
		Scan(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "usuario o contraseña incorrectos"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "usuario o contraseña incorrectos"})
		return
	}

	token, err := generateToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno"})
		return
	}

	session := &models.Session{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
	}
	if _, err := s.db.NewInsert().Model(session).Exec(context.Background()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "error creando sesión"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":    token,
		"username": user.Username,
	})
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
