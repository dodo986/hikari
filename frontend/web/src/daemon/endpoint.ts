import { resolveEndpoint } from './config'
import type { EndpointConfig } from './types'

let current: EndpointConfig | null = null

export function rememberEndpoint(endpoint: EndpointConfig): void {
  current = endpoint
}

export function endpoint(): EndpointConfig | null {
  return current
}

/** ws://host:port/rpc becomes http://host:port, the bridge art is served on. */
function bridgeBase(config: EndpointConfig): string {
  const url = new URL(config.url)
  return `${url.protocol === 'wss:' ? 'https' : 'http'}://${url.host}`
}

/**
 * A cached cover is a path on disk; a browser cannot read it, so it goes
 * through the daemon's own loopback route. Remote URLs pass straight through.
 */
export function artUrl(value: string | undefined, width = 0): string {
  if (!value) return ''
  if (value.startsWith('http://') || value.startsWith('https://') || value.startsWith('data:')) return value
  const config = current
  if (!config) return ''
  const clean = value.replace(/^file:\/\//, '')
  const name = clean.slice(clean.lastIndexOf('/') + 1)
  if (!name) return ''
  const size = width > 0 ? `&w=${Math.min(1024, Math.round(width))}` : ''
  return `${bridgeBase(config)}/art/${encodeURIComponent(name)}?token=${encodeURIComponent(config.token)}${size}`
}

export async function loadEndpoint(): Promise<EndpointConfig> {
  const config = await resolveEndpoint()
  rememberEndpoint(config)
  return config
}
