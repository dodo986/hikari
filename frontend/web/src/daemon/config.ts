import { hasEndpointSource, liveEndpoint } from '../host/bridge'
import type { EndpointConfig } from './types'

declare global {
  interface Window {
    __HIKARI_DAEMON__?: EndpointConfig
  }
}

export async function resolveEndpoint(): Promise<EndpointConfig> {
  if (hasEndpointSource()) {
    const endpoint = await liveEndpoint()
    if (!endpoint.url || !endpoint.token) throw new Error('the daemon endpoint is incomplete')
    return endpoint
  }
  const injected = window.__HIKARI_DAEMON__
  if (injected?.url) return injected
  const response = await fetch('/__daemon.json', {
    cache: 'no-store',
    signal: AbortSignal.timeout(4000),
  })
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string }
    throw new Error(body.error ?? 'the daemon endpoint is not available')
  }
  const config = (await response.json()) as EndpointConfig
  if (!config.url || !config.token) throw new Error('the daemon endpoint is incomplete')
  return config
}
