import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { loadEndpoint } from './daemon/endpoint'
import { applyPalette } from './theme/palette'
import './theme/global.css'

// A right to left build is one variable: the host passes the direction in.
const hostDir = (window as unknown as { __HIKARI_DIR__?: string }).__HIKARI_DIR__
if (hostDir === 'rtl' || hostDir === 'ltr') document.documentElement.dir = hostDir

const root = document.getElementById('root')
if (!root) throw new Error('the page has no #root element')

// The palette has to be on the first paint; a missing daemon is not fatal.
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
