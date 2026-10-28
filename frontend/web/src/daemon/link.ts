import { RpcClient } from './client'
import { loadEndpoint } from './endpoint'
import { gameKey, useDaemon } from './store'
import { errorMessage } from '../lib/errors'
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

export const rpc = new RpcClient(async () => {
  try {
    const config = await loadEndpoint()
    useDaemon.getState().setError(null)
    return config
  } catch (error) {
    // The offline screen quotes this, so a stale daemon reads as what it is.
    useDaemon.getState().setError(errorMessage(error))
    throw error
  }
}, (connection) => {
  useDaemon.getState().setConnection(connection)
  if (connection === 'live') {
    void refreshLibrary().catch(() => {})
    void refreshAuth().catch(() => {})
    void refreshStatus().catch(() => {})
  }
})

const EVENTS = [
  'library.changed',
  'install.progress',
  'game.state',
  'auth.changed',
  'mods.progress',
  'mods.log',
  'mods.auth_url',
  'mods.done',
]

/** Returns the unsubscribe, so a remount cannot stack a second handler set. */
export function startDaemonLink(): () => void {
  const offs = [
    rpc.on('library.changed', (params) => {
      useDaemon.getState().applyLibraryChanged((params ?? {}) as LibraryChangedEvent)
      void refreshLibrary().catch(() => {})
    }),
    rpc.on('install.progress', (params) => {
      if (params) useDaemon.getState().applyProgress(params as InstallProgress)
    }),
    rpc.on('game.state', (params) => {
      if (params) useDaemon.getState().applyGameState(params as GameStateEvent)
    }),
    rpc.on('auth.changed', () => {
      void refreshAuth().catch(() => {})
    }),
  ]
  rpc.start()
  rpc.subscribe(EVENTS)
  return () => offs.forEach((off) => off())
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
