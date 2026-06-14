package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"io/fs"
	"net/http"
	"strconv"

	"allium-server/internal/ui"
	"allium-server/internal/downloads"
)

// handleListPhotos devuelve una página de fotos ordenadas por fecha de captura.
//
// Query params:
//
//	limit  int — cuántas fotos devolver (default: 50, max: 200)
//	offset int — cuántas omitir al inicio (para paginación)
//
// Response 200: { "photos": [...], "total": N }
// Response 500: { "error": "..." }
func (s *Server) handleListPhotos(w http.ResponseWriter, r *http.Request) {
	// Valores por defecto si el cliente no manda nada
	limit := 50
	offset := 0

	// r.URL.Query() parsea el query string de la URL (?limit=20&offset=40)
	// .Get("limit") devuelve "" si no existe el parámetro
	if v := r.URL.Query().Get("limit"); v != "" {
		// strconv.Atoi convierte "20" → 20; si no es un número válido ignoramos
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			// min() evita que un cliente pida 99999 fotos de golpe
			limit = min(n, 200)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// r.Context() lleva el contexto de la request HTTP: si el cliente
	// cierra la conexión, el contexto se cancela y la query de BD se aborta
	photos, total, err := s.photoRepo.ListPaginated(r.Context(), limit, offset)
	if err != nil {
		// 500 solo si es un error interno real, no un "sin resultados"
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// map[string]any construye el JSON de respuesta:
	// { "photos": [...], "total": 142 }
	// El frontend usa "total" para saber cuántas páginas hay
	writeJSON(w, http.StatusOK, map[string]any{
		"photos": photos,
		"total":  total,
	})
}

// handleGetPhoto devuelve los metadatos completos de una foto por ID.
//
// Path param: id int64
// Response 200: { "photo": {...} }
// Response 404: { "error": "not found" }
func (s *Server) handleGetPhoto(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	// 1. id := r.PathValue("id") → strconv.ParseInt(id, 10, 64)
	// 2. s.photoRepo.GetByID(r.Context(), id)
	// 3. Si sql.ErrNoRows → 404, si otro error → 500
	id := r.PathValue("id")
	photoID, err := strconv.ParseInt(id, 10, 64)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "There was an error converting the id to number"})
		return
	}

	photo, err := s.photoRepo.GetByID(r.Context(), photoID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "There was no photo with that id"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Something went wrong"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"photo": photo})
}

// handleServeThumb sirve el archivo de thumbnail WebP directamente.
// Usa http.ServeFile para beneficiarse del soporte de Range requests y ETag.
//
// Path param: id int64
// Response 200: imagen WebP
// Response 404: si el thumbnail no existe en disco
func (s *Server) handleServeThumb(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	// 1. Obtener la photo de la BD por ID
	// 2. http.ServeFile(w, r, photo.ThumbPath)
	id := r.PathValue("id")
	photoID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Incorrect value for id", http.StatusBadRequest)
		return
	}

	photo, err := s.photoRepo.GetByID(r.Context(), photoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "No photo with that id", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, photo.ThumbPath)
}

// handleListAlbums devuelve todos los álbumes con su foto de portada.
//
// Response 200: { "albums": [...] }
func (s *Server) handleListAlbums(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	// 1. Consultar tabla albums con Bun, precargar relación CoverPhoto
	// 2. Devolver JSON
	offset := 0
	limit := 50

	if v := r.URL.Query().Get("limit"); v != "" {
		// strconv.Atoi convierte "20" → 20; si no es un número válido ignoramos
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			// min() evita que un cliente pida 99999 fotos de golpe
			limit = min(n, 200)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	albums, total, err := s.albumRepo.ListPaginated(r.Context(), limit, offset)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "There was an error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"albums": albums, "total": total})
}

// handleIngest dispara una ingesta de Google Takeout y devuelve progreso via SSE.
// El cliente debe conectarse con EventSource en el frontend para recibir eventos.
//
// Body JSON: { "source_path": "/ruta/al/takeout.zip", "dry_run": false }
// Response: text/event-stream con eventos de tipo "progress" y "done"
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourcePath string `json:"source_path"`
		DryRun     bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.SourcePath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source_path is required"})
		return
	}

	ingester := downloads.NewIngester(downloads.IngestConfig{
		SourcePath:  req.SourcePath,
		DestDataDir: s.dataDir,
		WorkerCount: 4,
		DryRun:      req.DryRun,
	})
	if err := ingester.Start(r.Context()); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	for event := range ingester.Progress {
		fmt.Fprint(w, "data: ")
		_ = enc.Encode(event)
		fmt.Fprint(w, "\n")
		flusher.Flush()
	}

	fmt.Fprint(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

// handleStatus devuelve el estado del servidor: uptime, dirección .onion,
// versión, estadísticas de la BD.
//
// Response 200: { "onion": "...", "total_photos": N, "version": "0.1.0" }
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	// 1. Obtener dirección .onion de s.torCtrl.OnionAddress()
	// 2. Contar fotos en BD
	// 3. Devolver JSON con estado
	onionAddress := s.torCtrl.OnionAddress()
	if onionAddress == ""{
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "pending",
			"onion":  onionAddress,
		})
		return 
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "onion": onionAddress})
}

// handleFrontend sirve el build estático de React.
// Redirige todas las rutas desconocidas al index.html para que React Router funcione.
func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	// Opción 1 (dev): proxy al servidor de Vite en :5173
	// Opción 2 (prod): http.FileServer(http.FS(embeddedUI)) donde embeddedUI son los
	//                  archivos de ui/dist embebidos con //go:embed

 	fsys, _ := fs.Sub(ui.Dist, "dist")
    // Archivos estáticos exactos se sirven directo; el resto → index.html (React Router)
    if _, err := fs.Stat(fsys, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
        r.URL.Path = "/"
    }
    http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
}

// writeJSON es un helper para escribir respuestas JSON con el Content-Type correcto.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
