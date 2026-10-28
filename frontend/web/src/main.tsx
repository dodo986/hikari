import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { loadEndpoint } from './daemon/endpoint'
import { applyPalette } from './theme/palette'
import './theme/global.css'

// A translation layer sets the document direction; the host can also pass one
// in, which keeps a right to left build a one variable change.
const hostDir = (window as unknown as { __HIKARI_DIR__?: string }).__HIKARI_DIR__
if (hostDir === 'rtl' || hostDir === 'ltr') document.documentElement.dir = hostDir

const root = document.getElementById('root')
if (!root) throw new Error('the page has no #root element')

// The endpoint carries the palette too, and tiles need it on the first paint,
// so it is resolved before anything renders. A missing daemon is not fatal: the
// app shows its offline screen and keeps reconnecting.
void loadEndpoint()
  .then((endpoint) => applyPalette(endpoint.palette))
  .catch(() => {})
  .finally(() => {
    createRoot(root).render(
      <StrictMode>
        <App />
      </StrictMode>,
    )
  })
