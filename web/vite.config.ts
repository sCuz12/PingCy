import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss()
  ],
  server: {
    proxy: {
      '/status': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/uptime': 'http://localhost:8080'
    }
  }

})
