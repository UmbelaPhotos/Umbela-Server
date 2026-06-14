/**
 * Tipos TypeScript para los modelos del dominio Allium.
 * Deben mantenerse en sincronía con los structs de Go en internal/models/*.go
 */

/** Representa una foto almacenada en el servidor */
export interface Photo {
  id: number;
  hash: string;        // SHA256 hex — identidad única de la foto
  title: string;
  description: string;
  capturedAt: number;  // Unix timestamp de cuándo fue tomada
  latitude: number;
  longitude: number;
  altitude: number;
  filePath: string;
  thumbPath: string;
  blurhash: string;    // Para el placeholder de carga borroso
  createdAt: string;   // ISO 8601
}

/** Representación de un álbum de fotos */
export interface Album {
  id: number;
  name: string;
  description: string;
  coverPhotoId: number | null;
  createdAt: string;
  updatedAt: string;
  photos?: Photo[];
}

/** Estado del servidor (respuesta de GET /api/status) */
export interface ServerStatus {
  onion: string;       // Dirección .onion (vacío si Tor no está activo)
  totalPhotos: number;
  version: string;
  torEnabled: boolean;
}

/** Respuesta paginada de fotos */
export interface PhotosResponse {
  photos: Photo[];
  total: number;
}

/** Respuesta de login */
export interface AuthResponse {
  token: string;
  username: string;
}

/** Evento de progreso de ingesta (via SSE) */
export interface IngestProgressEvent {
  total: number;
  processed: number;
  skipped: number;
  errors: number;
  current: string;  // Nombre del archivo que se está procesando
}
