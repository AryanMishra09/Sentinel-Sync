import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    // Proxy so the dev server can reach replicas without CORS issues during dev.
    proxy: {
      '/api-a': { target: 'http://localhost:8080', rewrite: (p) => p.replace(/^\/api-a/, '') },
      '/api-b': { target: 'http://localhost:8081', rewrite: (p) => p.replace(/^\/api-b/, '') },
      '/api-c': { target: 'http://localhost:8082', rewrite: (p) => p.replace(/^\/api-c/, '') },
    },
  },
})
