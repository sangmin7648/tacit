import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// dist/ is embedded into the app binary (cmd/tacit-app/main.go). public/
// carries dist/.gitkeep across the rebuild that empties dist/, keeping the
// tracked placeholder the Go embed needs when no frontend has been built.
export default defineConfig({
  plugins: [svelte()],
  build: { outDir: 'dist', emptyOutDir: true },
})
