import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],

  // Proxy de desarrollo: redirige /api/* al servidor Go que corre en :8080
  // Así el frontend no tiene problemas de CORS durante el desarrollo.
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        // Activar si el servidor Go requiere HTTPS (poco probable en dev)
        // secure: false,
      },
    },
  },

  // En producción, el build se genera en ui/dist y el servidor Go lo sirve estático
  build: {
    outDir: '../internal/ui/dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: (id) => {
          if (id.includes('node_modules/react') || id.includes('node_modules/react-dom')) {
            return 'react';
          }
        },
      },
    },
  },
})
