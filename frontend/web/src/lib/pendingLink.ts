import { modsGames, modsInstall } from '../daemon/mods'
import { refreshMods } from '../daemon/modsLink'
import { navigate } from '../router'
import { gameForNxm, parseNxm } from './nxm'

declare global {
  interface Window {
    /** Set by the host when the desktop opened the window with a link. */
    __HIKARI_LINK__?: string
  }
}

/**
 * A browser hands nxm:// links to this window. The daemon install is started
 * first and the page follows it, so the user sees the job they asked for.
 */
export async function handlePendingLink(): Promise<string> {
  const raw = window.__HIKARI_LINK__ ?? ''
  const link = parseNxm(raw)
  if (!link) return ''
  const { games } = await modsGames()
  const game = gameForNxm(link, games ?? [])
  if (!game) return `no game in the backend matches ${link.domain}`
  // Open the game first: the job shows up there even if the request fails.
  navigate({ page: 'mods', game })
  await modsInstall({ game }, link.url)
  void refreshMods()
  return ''
}
