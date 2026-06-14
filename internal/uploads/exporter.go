// Package uploads maneja la exportación de fotos desde Allium hacia Google,
// permitiendo a los usuarios "llevarse" sus fotos de vuelta a la nube si lo desean.
// Este paquete está reservado para implementación futura.
//
// Diseño previsto:
//   - Autenticación OAuth2 con Google Photos API
//   - Selección de álbumes a exportar
//   - Rate limiting para respetar los límites de la API de Google
//   - Progreso reportado via canal (similar a downloads.Ingester)
package uploads

import (
	"context"
	"fmt"
)

// Exporter gestiona la exportación de fotos hacia servicios externos.
// Por ahora solo Google Photos está planificado, pero la interfaz
// permite extensiones futuras (Flickr, Nextcloud, etc.).
type Exporter struct {
	// TODO: añadir cliente OAuth2, destino, etc.
	destination string
}

// NewExporter crea un Exporter configurado para el destino dado.
//
// Input:  destination string — "google_photos" u otros futuros
// Output: *Exporter, error si el destino no está soportado
func NewExporter(destination string) (*Exporter, error) {
	// TODO: Implementar
	// 1. Switch en destination para seleccionar el backend correcto
	// 2. Inicializar credenciales OAuth2
	return nil, fmt.Errorf("not implemented: uploads está reservado para implementación futura")
}

// Export exporta los IDs de fotos indicados al servicio destino configurado.
//
// Input:  ctx context.Context
//         photoIDs []int64 — IDs de fotos en la BD local a exportar
// Output: error si la exportación falla
func (e *Exporter) Export(ctx context.Context, photoIDs []int64) error {
	// TODO: Implementar cuando se requiera
	return fmt.Errorf("not implemented")
}
