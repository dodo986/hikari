import type { EndpointConfig } from './types'

declare global {
  interface Window {
    __HIKARI_DAEMON__?: EndpointConfig
  }
}

// The host injects the endpoint in the window, so a reload always gets a fresh token;
// the Vite dev server answers the same file in a plain browser.
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
