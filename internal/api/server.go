package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"allium-server/internal/image"
	"allium-server/internal/models"
	"allium-server/internal/storage"
	"allium-server/internal/tor"

	"github.com/uptrace/bun"
)

type Server struct {
	port      int
	dataDir   string
	router    *http.ServeMux
	db        *bun.DB
	photoRepo *storage.PhotoRepository
	albumRepo *storage.AlbumRepository
	processor *image.Processor
	torCtrl   *tor.Controller
}

func NewServer(port int, dataDir string, db *bun.DB, photoRepo *storage.PhotoRepository, albumRepo *storage.AlbumRepository) *Server {
	s := &Server{
		port:      port,
		dataDir:   dataDir,
		router:    http.NewServeMux(),
		db:        db,
		photoRepo: photoRepo,
		albumRepo: albumRepo,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Auth — sin protección (son los endpoints de login/registro)
	s.router.HandleFunc("POST /api/auth/register", s.handleRegister)
	s.router.HandleFunc("POST /api/auth/login", s.handleLogin)

	// Rutas protegidas con token
	s.router.HandleFunc("GET /api/photos", s.auth(s.handleListPhotos))
	s.router.HandleFunc("GET /api/photos/{id}", s.auth(s.handleGetPhoto))
	s.router.HandleFunc("GET /api/photos/{id}/thumb", s.auth(s.handleServeThumb))
	s.router.HandleFunc("GET /api/albums", s.auth(s.handleListAlbums))
	s.router.HandleFunc("GET /api/status", s.auth(s.handleStatus))
	s.router.HandleFunc("POST /api/ingest", s.auth(s.handleIngest))

	s.router.HandleFunc("/", s.handleFrontend)
}

// auth es el middleware que valida el token Bearer en cada request protegida.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no autenticado"})
			return
		}

		var session models.Session
		err := s.db.NewSelect().Model(&session).
			Where("token = ?", token).
			Scan(context.Background())
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "sesión inválida"})
			return
		}

		next(w, r)
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	return http.ListenAndServe(addr, s.router)
}
