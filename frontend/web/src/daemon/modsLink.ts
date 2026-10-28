import { rpc } from './link'
import { useDaemon } from './store'
import { openUrl, pickFolder } from '../host/bridge'
import { modsAccount, modsConfigure, modsGames, modsStatus } from './mods'
import type { ModsAccount, ModsDone, ModsGame, ModsProgress } from './types'
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

/** Pick a folder for a game and persist it; both the list and the page use it. */
export async function chooseModsFolder(
  game: ModsGame,
  label: string,
  saved: string,
  onNote: (note: string) => void,
  onFailed: (message: string) => void,
): Promise<void> {
  const path = await pickFolder(label)
  if (!path) return
  try {
    const result = await modsConfigure(game.id, path)
    if (result.problems?.length) onFailed(result.problems.join('; '))
    else onNote(`${game.name}: ${saved}`)
    await refreshMods()
  } catch (error) {
    onFailed(errorMessage(error))
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
