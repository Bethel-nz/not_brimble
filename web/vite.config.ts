import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'
import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    devtools(),
    tailwindcss(),
    TanStackRouterVite(),
    viteReact(),
  ],
  server: {
    host: true,
    port: 3000,
    proxy: {
      // VITE_API_TARGET is set to http://api:8080 by docker-compose.dev.yml
      // so the Vite container can reach the API over the shared network.
      // When running `bun run dev` on the host, the default works against
      // an API exposed on localhost:8080.
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
        ws: true,
      },
    },
  },
})
