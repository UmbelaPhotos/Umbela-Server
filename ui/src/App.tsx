import { useState, useEffect } from 'react';
import './index.css';
import { PhotoCard } from './components/PhotoCard';
import { Lightbox } from './components/Lightbox';
import { StatusBar } from './components/StatusBar';
import { AuthPage } from './components/AuthPage';
import { fetchPhotos, fetchStatus } from './api';
import type { Photo, ServerStatus } from './types';

const TOKEN_KEY = 'allium_token';

type View = 'photos' | 'albums' | 'favorites';

const SIDEBAR_ITEMS: { id: View; icon: string; label: string }[] = [
  { id: 'photos',    icon: '🖼',  label: 'Fotos' },
  { id: 'albums',    icon: '📁',  label: 'Álbumes' },
  { id: 'favorites', icon: '⭐', label: 'Favoritos' },
];

function groupByDate(photos: Photo[]): { label: string; photos: Photo[] }[] {
  const map = new Map<string, Photo[]>();
  for (const p of photos) {
    const d = p.capturedAt
      ? new Date(p.capturedAt * 1000).toLocaleDateString('es-MX', {
          weekday: 'long', year: 'numeric', month: 'long', day: 'numeric',
        })
      : 'Sin fecha';
    if (!map.has(d)) map.set(d, []);
    map.get(d)!.push(p);
  }
  return Array.from(map.entries()).map(([label, photos]) => ({ label, photos }));
}

function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY));
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [total, setTotal] = useState(0);
  const [selectedPhoto, setSelectedPhoto] = useState<Photo | null>(null);
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<View>('photos');
  const [search, setSearch] = useState('');

  const handleAuth = (newToken: string) => {
    localStorage.setItem(TOKEN_KEY, newToken);
    setToken(newToken);
  };

  const handleLogout = () => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
  };

  useEffect(() => {
    fetchStatus().then(setStatus).catch(console.error);
  }, []);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    fetchPhotos(200, 0)
      .then(({ photos, total }) => { setPhotos(photos); setTotal(total); })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [token]);

  if (!token) return <AuthPage onAuth={handleAuth} />;

  const filtered = search.trim()
    ? photos.filter((p) =>
        p.title?.toLowerCase().includes(search.toLowerCase()) ||
        p.description?.toLowerCase().includes(search.toLowerCase())
      )
    : photos;

  const groups = groupByDate(filtered);

  return (
    <div className="app">
      {/* NAVBAR */}
      <nav className="navbar">
        <div className="navbar-logo">
          <span className="navbar-logo-icon">🧅</span>
          <span>Allium</span>
        </div>

        <div className="search-bar">
          <svg className="search-bar-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" />
          </svg>
          <input
            type="search"
            placeholder="Buscar fotos"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>

        <div className="navbar-end">
          <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>
            {total > 0 ? `${total} fotos` : ''}
          </span>
          <button className="btn-icon btn-logout" onClick={handleLogout} title="Cerrar sesión">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
              <polyline points="16 17 21 12 16 7"/>
              <line x1="21" y1="12" x2="9" y2="12"/>
            </svg>
          </button>
        </div>
      </nav>

      <div className="main-content">
        <div className="layout">
          {/* SIDEBAR */}
          <aside className="sidebar">
            {SIDEBAR_ITEMS.map((item) => (
              <div
                key={item.id}
                className={`sidebar-item ${view === item.id ? 'active' : ''}`}
                onClick={() => setView(item.id)}
              >
                <span className="sidebar-icon">{item.icon}</span>
                {item.label}
              </div>
            ))}
          </aside>

          {/* GALERÍA */}
          <main className="gallery-area">
            {loading && (
              <div className="empty-state">
                <div className="empty-state-icon" style={{ animation: 'pulse 1.4s infinite' }}>🧅</div>
                <p>Cargando tu galería...</p>
              </div>
            )}

            {error && (
              <div className="empty-state">
                <div className="empty-state-icon">⚠️</div>
                <h2>Sin conexión</h2>
                <p>{error}</p>
              </div>
            )}

            {!loading && !error && filtered.length === 0 && (
              <div className="empty-state">
                <div className="empty-state-icon">📷</div>
                <h2>{search ? 'Sin resultados' : 'Galería vacía'}</h2>
                <p>
                  {search
                    ? `No hay fotos que coincidan con "${search}".`
                    : 'Importa tus fotos usando un export de Google Takeout para comenzar.'}
                </p>
                {!search && (
                  <button className="btn btn-primary" disabled>
                    Importar Google Takeout
                  </button>
                )}
              </div>
            )}

            {!loading && !error && filtered.length > 0 && groups.map((group) => (
              <div key={group.label} className="date-group">
                <div className="date-group-header">{group.label}</div>
                <div className="gallery-grid">
                  {group.photos.map((photo) => (
                    <PhotoCard
                      key={photo.id}
                      photo={photo}
                      onOpen={setSelectedPhoto}
                    />
                  ))}
                </div>
              </div>
            ))}
          </main>
        </div>
      </div>

      <Lightbox photo={selectedPhoto} onClose={() => setSelectedPhoto(null)} />
      <StatusBar status={status} />
    </div>
  );
}

export default App;
