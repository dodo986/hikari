import type { ModsGame } from '../daemon/types'

export interface NxmLink {
  domain: string
  modId: number
  fileId: number
  url: string
}

// The query carries a short lived download key, so the url is never rebuilt from parts.
export function parseNxm(link: string): NxmLink | null {
  if (!link.startsWith('nxm://')) return null
  const rest = link.slice('nxm://'.length)
  const path = rest.split('?')[0] ?? ''
  const parts = path.split('/').filter((part) => part.length > 0)
  if (parts.length < 5 || parts[1] !== 'mods' || parts[3] !== 'files') return null
  const domain = parts[0] ?? ''
  const modId = Number(parts[2])
  const fileId = Number(parts[4] ?? 0)
  if (domain === '' || !Number.isFinite(modId) || modId <= 0) return null
  return { domain, modId, fileId: Number.isFinite(fileId) ? fileId : 0, url: link }
}

export function gameForNxm(link: NxmLink, games: ModsGame[]): string | null {
  const match = games.find((game) => game.nexus_domain === link.domain)
  return match ? match.id : null
}
