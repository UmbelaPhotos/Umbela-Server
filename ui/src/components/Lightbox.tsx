import { useEffect } from 'react';
import type { Photo } from '../types';

interface LightboxProps {
  photo: Photo | null;
  onClose: () => void;
}

export function Lightbox({ photo, onClose }: LightboxProps) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);

  if (!photo) return null;

  const capturedDate = photo.capturedAt
    ? new Date(photo.capturedAt * 1000).toLocaleDateString('es-MX', {
        weekday: 'long', year: 'numeric', month: 'long', day: 'numeric',
      })
    : null;

  return (
    <div
      className="lightbox-backdrop"
      onClick={onClose}
      role="dialog"
      aria-modal
      aria-label={photo.title || 'Foto'}
    >
      {/* Toolbar superior */}
      <div className="lightbox-toolbar" onClick={(e) => e.stopPropagation()}>
        <button className="lightbox-close" onClick={onClose} aria-label="Cerrar">
          ✕
        </button>
        {photo.title && (
          <span style={{ color: 'white', fontSize: '0.9rem', fontWeight: 500 }}>
            {photo.title}
          </span>
        )}
      </div>

      {/* Imagen */}
      <div className="lightbox-content" onClick={(e) => e.stopPropagation()}>
        <img
          src={`/api/photos/${photo.id}/thumb`}
          alt={photo.title || `Foto ${photo.id}`}
        />
      </div>

      {/* Metadatos inferiores */}
      {(photo.title || capturedDate) && (
        <div className="lightbox-meta" onClick={(e) => e.stopPropagation()}>
          {photo.title    && <p className="lightbox-meta-title">{photo.title}</p>}
          {capturedDate   && <p className="lightbox-meta-date">{capturedDate}</p>}
        </div>
      )}
    </div>
  );
}
