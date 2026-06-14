// Package storage maneja el acceso a datos (CRUD) para los modelos del dominio.
// Cada archivo en este paquete contiene un "repositorio" para un modelo específico.
// Los repositorios son la ÚNICA capa que habla directamente con *bun.DB.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"allium-server/internal/models"

	"github.com/uptrace/bun"
)

// PhotoRepository encapsula todas las operaciones de BD para el modelo Photo.
type PhotoRepository struct {
	db *bun.DB
}

// NewPhotoRepository crea una nueva instancia del repositorio de fotos.
//
// Input:  *bun.DB — conexión activa a la base de datos
// Output: *PhotoRepository listo para usar
func NewPhotoRepository(db *bun.DB) *PhotoRepository {
	return &PhotoRepository{db: db}
}

// Save inserta una nueva Photo en la BD. Si ya existe una foto con el mismo
// Hash (deduplicación), devuelve el registro existente sin error.
//
// Input:  ctx context.Context, photo *models.Photo con todos los campos requeridos
// Output: error si la inserción falla por razón distinta a duplicado
func (r *PhotoRepository) Save(ctx context.Context, photo *models.Photo) error {
	_, err := r.db.NewInsert().Model(photo).On("CONFLICT (hash) DO NOTHING").Exec(ctx)
	return err
}

// GetByID busca una foto por su ID primario.
//
// Input:  ctx context.Context, id int64
// Output: *models.Photo encontrada, o error si no existe / fallo de BD
func (r *PhotoRepository) GetByID(ctx context.Context, id int64) (*models.Photo, error) {
	// TODO: Implementar
	// 1. photo := new(models.Photo)
	// 2. db.NewSelect().Model(photo).Where("id = ?", id).Scan(ctx)
	// 3. Devolver photo o error
	photo := new(models.Photo)
	err := r.db.NewSelect().Model(photo).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { //errors.Is es para chaecar si un error es de un tipo. sql.ErrNoRows es que no hay
			return nil, fmt.Errorf("Photo with id %d not found: %w", id, err)
		}
		return nil, fmt.Errorf("There was an error trying to get the image you requested: %w", err)
	}
	return photo, nil
}

// GetByHash busca una foto por su hash SHA256 (para deduplicación).
//
// Input:  ctx context.Context, hash string — SHA256 hexadecimal del archivo original
// Output: *models.Photo si existe, nil + nil si no existe
func (r *PhotoRepository) GetByHash(ctx context.Context, hash string) (*models.Photo, error) {
	photo := new(models.Photo)
	err := r.db.NewSelect().Model(photo).Where("hash = ?", hash).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return photo, nil
}

// ListPaginated devuelve una página de fotos ordenadas por fecha de captura descendente.
//
// Input:  ctx, limit int (fotos por página), offset int (skip)
// Output: slice de fotos, total de fotos en BD, error
func (r *PhotoRepository) ListPaginated(ctx context.Context, limit int, offset int) ([]*models.Photo, int, error) {
	// TODO: Implementar
	// 1. db.NewSelect().Model(&photos).OrderExpr("captured_at DESC").Limit(limit).Offset(offset).ScanAndCount(ctx)
	// 2. Devolver fotos, total, nil
	photos := []*models.Photo{}
	total, err := r.db.NewSelect().Model(&photos).OrderExpr("captured_at DESC").Limit(limit).Offset(offset).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("Error getting the images: %w", err)
	}
	return photos, total, nil
}

// Delete elimina una foto de la BD por su ID.
// NOTA: No borra el archivo físico; eso es responsabilidad del caller.
//
// Input:  ctx, id int64
// Output: error si no existe o fallo de BD
func (r *PhotoRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.NewDelete().Model((*models.Photo)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error deleting album %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("album %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}
