import { rpc } from './link'
import { useDaemon } from './store'
import { openUrl } from '../host/bridge'
import { modsAccount, modsGames, modsStatus } from './mods'
import type { ModsAccount, ModsDone, ModsProgress } from './types'
import { errorMessage } from '../lib/errors'

// status, games and account share one error slot; the page reads them together.
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
    state.setMods({ error: errorMessage(error) })
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

/** Returns the unsubscribe, so a remount cannot stack a second handler set. */
export function startModsLink(): () => void {
  const state = useDaemon.getState()

  const offs = [
    rpc.on('mods.progress', (params) => {
      if (params) state.setMods({ job: params as ModsProgress })
    }),
    rpc.on('mods.log', (params) => {
      const event = (params ?? {}) as { line?: string }
      if (event.line) state.pushModsLog(event.line)
    }),
    rpc.on('mods.auth_url', (params) => {
      const event = (params ?? {}) as { url?: string }
      if (!event.url) return
      state.setMods({ authUrl: event.url })
      void openUrl(event.url)
    }),
    rpc.on('mods.done', (params) => {
      const event = (params ?? {}) as ModsDone
      state.setMods({ job: null, authUrl: '', log: [] })
      if (event.ok) {
        void refreshMods()
        return
      }
      if (event.error) state.setMods({ error: event.error })
    }),
  ]
  return () => offs.forEach((off) => off())
}
