import type { ServerStatus } from '../types';

/**
 * StatusBar — Barra inferior que muestra el estado de la red Tor.
 *
 * Indica si el servidor está conectado a Tor y muestra la dirección .onion
 * cuando está disponible. El usuario puede copiarla al portapapeles.
 *
 * @param status - Estado del servidor (null = cargando)
 */
interface StatusBarProps {
  status: ServerStatus | null;
}

export function StatusBar({ status }: StatusBarProps) {
  const copyOnion = () => {
    if (status?.onion) {
      navigator.clipboard.writeText(status.onion);
      // TODO: Mostrar un toast de confirmación
    }
  };

  const dotClass = !status
    ? 'status-dot pending'
    : status.torEnabled && status.onion
    ? 'status-dot'
    : 'status-dot offline';

  return (
    <footer className="status-bar" id="status-bar">
      <div className={dotClass} aria-hidden="true" />

      {!status && <span>Conectando con el servidor...</span>}

      {status && !status.torEnabled && (
        <span>Tor desactivado — acceso solo local</span>
      )}

      {status && status.torEnabled && !status.onion && (
        <span className="pending">⏳ Esperando a que Tor bootstrappee...</span>
      )}

      {status?.onion && (
        <>
          <span>🧅 Activo en Tor:</span>
          <code
            style={{ color: 'var(--color-accent)', cursor: 'pointer' }}
            onClick={copyOnion}
            title="Click para copiar"
            id="onion-address"
          >
            {status.onion}
          </code>
          <span style={{ opacity: 0.5 }}>(click para copiar)</span>
        </>
      )}

      <span style={{ marginLeft: 'auto', opacity: 0.4, fontSize: '0.75rem' }}>
        {status?.totalPhotos ?? '—'} fotos · v{status?.version ?? '...'}
      </span>
    </footer>
  );
}
