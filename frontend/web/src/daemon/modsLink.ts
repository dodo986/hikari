import { rpc } from './link'
import { useDaemon } from './store'
import { modsAccount, modsGames, modsStatus } from './mods'
import type { ModsAccount, ModsDone, ModsGame, ModsProgress, ModsStatus } from './types'

/** mods.status and mods.games always travel together and share one error slot. */
export async function refreshMods(): Promise<void> {
  const state = useDaemon.getState()
  try {
    const [status, games, account] = await Promise.all([modsStatus(), modsGames(), modsAccount()])
    state.setMods({
      status,
      games: games.games ?? [],
      account,
      error: '',
    })
  } catch (error) {
    state.setMods({ error: error instanceof Error ? error.message : String(error) })
  }
}

export async function refreshModsAccount(): Promise<ModsAccount | null> {
  try {
    const account = await modsAccount()
    useDaemon.getState().setMods({ account })
    return account
  } catch {
    return null
  }
}

export async function refreshModsStatus(): Promise<ModsStatus | null> {
  try {
    const status = await modsStatus()
    useDaemon.getState().setMods({ status })
    return status
  } catch {
    return null
  }
}

export async function refreshModsGames(): Promise<ModsGame[]> {
  try {
    const games = await modsGames()
    useDaemon.getState().setMods({ games: games.games ?? [] })
    return games.games ?? []
  } catch {
    return []
  }
}

/** Wire events onto the store; the rail badge and the page read the same slice. */
export function startModsLink(): void {
  const state = useDaemon.getState()

  rpc.on('mods.progress', (params) => {
    const progress = params as ModsProgress
    state.setMods({ job: progress })
  })

  rpc.on('mods.log', (params) => {
    const event = params as { line?: string }
    if (event.line) state.pushModsLog(event.line)
  })

  rpc.on('mods.auth_url', (params) => {
    const event = params as { url?: string }
    if (!event.url) return
    state.setMods({ authUrl: event.url })
    window.open(event.url, '_blank', 'noopener')
  })

  rpc.on('mods.done', (params) => {
    const event = params as ModsDone
    state.setMods({ job: null, authUrl: '', log: [] })
    if (event.ok) {
      void refreshMods()
      return
    }
    if (event.error) state.setMods({ error: event.error })
  })
}

export const modsBusy = (): boolean => Boolean(useDaemon.getState().mods.job)
