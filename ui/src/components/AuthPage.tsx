import { useState } from 'react';
import { login, register } from '../api';

interface AuthPageProps {
  onAuth: (token: string) => void;
}

type Mode = 'login' | 'register';

export function AuthPage({ onAuth }: AuthPageProps) {
  const [mode, setMode] = useState<Mode>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const switchMode = (next: Mode) => {
    setMode(next);
    setError(null);
    setSuccess(null);
    setUsername('');
    setPassword('');
    setConfirmPassword('');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    if (!username.trim() || !password) {
      setError('Completa todos los campos.');
      return;
    }

    if (mode === 'register') {
      if (password !== confirmPassword) {
        setError('Las contraseñas no coinciden.');
        return;
      }
      if (password.length < 8) {
        setError('La contraseña debe tener al menos 8 caracteres.');
        return;
      }
    }

    setLoading(true);
    try {
      if (mode === 'login') {
        const { token } = await login(username.trim(), password);
        onAuth(token);
      } else {
        await register(username.trim(), password);
        setSuccess('Cuenta creada. Ahora puedes iniciar sesión.');
        switchMode('login');
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Error desconocido.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-bg">
      {/* Orbs decorativos de fondo */}
      <div className="auth-orb auth-orb-1" />
      <div className="auth-orb auth-orb-2" />
      <div className="auth-orb auth-orb-3" />

      <div className="auth-card animate-slide-up">
        {/* Logo */}
        <div className="auth-logo">
          <span className="auth-logo-icon">🧅</span>
          <span className="auth-logo-text">Allium</span>
        </div>

        {/* Tabs */}
        <div className="auth-tabs">
          <button
            className={`auth-tab ${mode === 'login' ? 'auth-tab-active' : ''}`}
            onClick={() => switchMode('login')}
            type="button"
          >
            Iniciar sesión
          </button>
          <button
            className={`auth-tab ${mode === 'register' ? 'auth-tab-active' : ''}`}
            onClick={() => switchMode('register')}
            type="button"
          >
            Crear cuenta
          </button>
        </div>

        <form className="auth-form" onSubmit={handleSubmit} noValidate>
          <div className="auth-field">
            <label className="auth-label" htmlFor="auth-username">
              Usuario
            </label>
            <input
              id="auth-username"
              className="auth-input"
              type="text"
              placeholder="nombre_de_usuario"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete={mode === 'login' ? 'username' : 'new-password'}
              autoFocus
              disabled={loading}
            />
          </div>

          <div className="auth-field">
            <label className="auth-label" htmlFor="auth-password">
              Contraseña
            </label>
            <input
              id="auth-password"
              className="auth-input"
              type="password"
              placeholder={mode === 'register' ? 'Mínimo 8 caracteres' : '••••••••'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              disabled={loading}
            />
          </div>

          {mode === 'register' && (
            <div className="auth-field animate-slide-up">
              <label className="auth-label" htmlFor="auth-confirm">
                Confirmar contraseña
              </label>
              <input
                id="auth-confirm"
                className="auth-input"
                type="password"
                placeholder="Repite la contraseña"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                autoComplete="new-password"
                disabled={loading}
              />
            </div>
          )}

          {error && (
            <div className="auth-alert auth-alert-error animate-slide-up">
              <span className="auth-alert-icon">⚠</span>
              {error}
            </div>
          )}

          {success && (
            <div className="auth-alert auth-alert-success animate-slide-up">
              <span className="auth-alert-icon">✓</span>
              {success}
            </div>
          )}

          <button
            className="auth-submit"
            type="submit"
            disabled={loading}
          >
            {loading ? (
              <span className="auth-spinner" />
            ) : mode === 'login' ? (
              'Entrar'
            ) : (
              'Crear cuenta'
            )}
          </button>
        </form>

        <p className="auth-footer-text">
          Tu galería privada, protegida con Tor.{' '}
          <span className="auth-footer-icon">🧅</span>
        </p>
      </div>
    </div>
  );
}
