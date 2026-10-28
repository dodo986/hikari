import { readFileSync, existsSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

// The dev server hands the page the same two facts the Wails host injects in
// production: where the daemon listens and which palette to paint.
function daemonBridge(): Plugin {
  const runtimeDir = process.env.XDG_RUNTIME_DIR ?? ''
  const endpointFile = process.env.HIKARI_WS_FILE ?? join(runtimeDir, 'hikari', 'ws.json')
  const paletteFile = join(homedir(), '.cache', 'ryoku', 'colors.json')

  const readJson = (path: string): unknown => {
    if (!existsSync(path)) return undefined
    try {
      return JSON.parse(readFileSync(path, 'utf8'))
    } catch {
      return undefined
    }
  }

  return {
    name: 'hikari-daemon-bridge',
    configureServer(server) {
      server.middlewares.use('/__daemon.json', (_req, res) => {
        const endpoint = readJson(endpointFile)
        res.setHeader('content-type', 'application/json')
        res.setHeader('cache-control', 'no-store')
        if (!endpoint) {
          res.statusCode = 503
          res.end(JSON.stringify({ error: 'hikarid is not running: ' + endpointFile + ' is missing' }))
          return
        }
        res.end(JSON.stringify({ ...endpoint, palette: readJson(paletteFile) ?? null }))
      })
    },
  }
}

export default defineConfig({
  plugins: [react(), daemonBridge()],
  resolve: { alias: { '@': new URL('./src', import.meta.url).pathname } },
  server: { port: 5173, strictPort: true, host: '127.0.0.1' },
  test: {
    environment: 'jsdom',
    include: ['test/**/*.test.ts'],
  },
  build: { outDir: 'dist', sourcemap: false, target: 'es2022' },
})
