import { modsGames, modsInstall } from '../daemon/mods'
import { refreshMods } from '../daemon/modsLink'
import { navigate } from '../router'
import { gameForNxm, parseNxm } from './nxm'

declare global {
  interface Window {
    __HIKARI_LINK__?: string
  }
}

// The job starts before the page shows, so the user lands on what they asked for.
export async function handlePendingLink(): Promise<string> {
  const raw = window.__HIKARI_LINK__ ?? ''
  const link = parseNxm(raw)
  if (!link) return ''
  const { games } = await modsGames()
  const game = gameForNxm(link, games ?? [])
  if (!game) return `no game in the backend matches ${link.domain}`
  navigate({ page: 'mods', game })
  await modsInstall({ game }, link.url)
  void refreshMods()
  return ''
}
