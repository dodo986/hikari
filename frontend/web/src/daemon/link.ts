import { RpcClient } from './client'
import { loadEndpoint } from './endpoint'
import { gameKey, useDaemon } from './store'
import type {
  AuthStatus,
  AuthStatusResult,
  DaemonStatus,
  Game,
  GameStateEvent,
  InstallProgress,
  LibraryChangedEvent,
  LibraryListResult,
} from './types'

/** The one client the app talks through. */
export const rpc = new RpcClient(loadEndpoint, (connection) => {
  useDaemon.getState().setConnection(connection)
  if (connection === 'live') {
    void refreshLibrary().catch(() => {})
    void refreshAuth().catch(() => {})
    void refreshStatus().catch(() => {})
  }
})

if (import.meta.env.DEV) {
  // The e2e tests read state from here instead of scraping the DOM.
  Object.assign(window, { __hikari: { rpc, useDaemon } })
}

const EVENTS = [
  'library.changed',
  'install.progress',
  'game.state',
  'auth.changed',
  'opti.progress',
  'wine.progress',
  'mods.progress',
  'mods.log',
  'mods.auth_url',
  'mods.done',
]

export function startDaemonLink(): void {
  rpc.on('library.changed', (params) => {
    const event = (params ?? {}) as LibraryChangedEvent
    useDaemon.getState().applyLibraryChanged(event)
    void refreshLibrary()
  })
  rpc.on('install.progress', (params) => {
    useDaemon.getState().applyProgress(params as InstallProgress)
  })
  rpc.on('game.state', (params) => {
    useDaemon.getState().applyGameState(params as GameStateEvent)
  })
  rpc.on('auth.changed', () => {
    void refreshAuth().catch(() => {})
  })
  rpc.start()
  rpc.subscribe(EVENTS)
}

export async function refreshLibrary(): Promise<Game[]> {
  const result = await rpc.call<LibraryListResult>('library.list', {})
  const games = result.games ?? []
  useDaemon.getState().setGames(games)
  for (const game of games) {
    const key = gameKey(game.store, game.id)
    if (useDaemon.getState().installs[key]?.state === 'done') {
      useDaemon.getState().clearProgress(key)
    }
  }
  return games
}

export async function refreshAuth(): Promise<Record<string, AuthStatus>> {
  const result = await rpc.call<AuthStatusResult>('auth.status', {})
  const stores = result.stores ?? {}
  useDaemon.getState().setAuth(stores)
  return stores
}

export async function refreshStatus(): Promise<DaemonStatus> {
  const status = await rpc.call<DaemonStatus>('daemon.status', {})
  useDaemon.getState().setStatus(status)
  return status
}
