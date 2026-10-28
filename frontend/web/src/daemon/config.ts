import type { EndpointConfig } from './types'

declare global {
  interface Window {
    __HIKARI_DAEMON__?: EndpointConfig
  }
}

// In the Wails window the host injects the endpoint, so a reload always picks
// up a fresh token. In a plain browser (dev) it comes from the Vite bridge.
export async function resolveEndpoint(): Promise<EndpointConfig> {
  const injected = window.__HIKARI_DAEMON__
  if (injected?.url) return injected
  const response = await fetch('/__daemon.json', { cache: 'no-store' })
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string }
    throw new Error(body.error ?? 'the daemon endpoint is not available')
  }
  const config = (await response.json()) as EndpointConfig
  if (!config.url || !config.token) throw new Error('the daemon endpoint is incomplete')
  return config
}
