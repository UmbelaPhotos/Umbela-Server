import { useState } from 'react';
import type { Photo } from '../types';
import { thumbUrl } from '../api';

interface PhotoCardProps {
  photo: Photo;
  onOpen: (photo: Photo) => void;
}

export function PhotoCard({ photo, onOpen }: PhotoCardProps) {
  const [loaded, setLoaded] = useState(false);

  return (
    <div
      className="photo-card"
      onClick={() => onOpen(photo)}
      role="button"
      tabIndex={0}
      aria-label={photo.title || 'Foto'}
      onKeyDown={(e) => e.key === 'Enter' && onOpen(photo)}
    >
      {!loaded && <div className="photo-card-placeholder" />}
      <img
        src={thumbUrl(photo.id)}
        alt={photo.title || `Foto ${photo.id}`}
        loading="lazy"
        onLoad={() => setLoaded(true)}
        style={{ display: loaded ? 'block' : 'none' }}
      />
      <div className="photo-card-overlay">
        {photo.title && <span className="photo-card-title">{photo.title}</span>}
      </div>
    </div>
  );
}
