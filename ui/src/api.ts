import type { Photo, Album, PhotosResponse, ServerStatus, AuthResponse } from './types';

const BASE = '/api';

const TOKEN_KEY = 'allium_token';

function authHeaders(): HeadersInit {
  const token = localStorage.getItem(TOKEN_KEY);
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function login(username: string, password: string): Promise<AuthResponse> {
  const res = await fetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Error ${res.status}`);
  }
  return res.json();
}

export async function register(username: string, password: string): Promise<void> {
  const res = await fetch(`${BASE}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Error ${res.status}`);
  }
}

export async function fetchPhotos(limit = 50, offset = 0): Promise<PhotosResponse> {
  const res = await fetch(`${BASE}/photos?limit=${limit}&offset=${offset}`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Error ${res.status}: ${await res.text()}`);
  return res.json();
}

export async function fetchPhoto(id: number): Promise<Photo> {
  const res = await fetch(`${BASE}/photos/${id}`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Photo ${id} not found`);
  return res.json();
}

export function thumbUrl(id: number): string {
  return `${BASE}/photos/${id}/thumb`;
}

export async function fetchAlbums(): Promise<Album[]> {
  const res = await fetch(`${BASE}/albums`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Error ${res.status}`);
  return res.json();
}

export async function fetchStatus(): Promise<ServerStatus> {
  const res = await fetch(`${BASE}/status`, { headers: authHeaders() });
  if (!res.ok) return { onion: '', totalPhotos: 0, version: '0.1.0-dev', torEnabled: false };
  return res.json();
}

export function startIngest(sourcePath: string, dryRun = false): EventSource {
  const url = new URL(`${BASE}/ingest`, window.location.origin);
  console.warn('startIngest: backend no implementado', { sourcePath, dryRun });
  return new EventSource(url.toString());
}
