import { create } from 'zustand'
import type {
  AuthStatus,
  ConnectionState,
  DaemonStatus,
  Game,
  GameStateEvent,
  InstallProgress,
  LibraryChangedEvent,
  ModsAccount,
  ModsGame,
  ModsProgress,
  ModsStatus,
} from './types'

export interface ModsState {
  status: ModsStatus | null
  games: ModsGame[]
  account: ModsAccount | null
  job: ModsProgress | null
  log: string[]
  authUrl: string
  error: string
}

export interface DaemonState {
  connection: ConnectionState
  games: Game[]
  status: DaemonStatus | null
  installs: Record<string, InstallProgress>
  running: Record<string, GameStateEvent>
  auth: Record<string, AuthStatus>
  error: string | null
  setConnection: (state: ConnectionState) => void
  setGames: (games: Game[]) => void
  setStatus: (status: DaemonStatus | null) => void
  setError: (error: string | null) => void
  applyProgress: (progress: InstallProgress) => void
  clearProgress: (key: string) => void
  applyGameState: (event: GameStateEvent) => void
  setAuth: (stores: Record<string, AuthStatus>) => void
  applyLibraryChanged: (event: LibraryChangedEvent) => void
  mods: ModsState
  setMods: (patch: Partial<ModsState>) => void
  pushModsLog: (line: string) => void
}

export const gameKey = (store: string, id: string): string => `${store}/${id}`

export const useDaemon = create<DaemonState>((set) => ({
  connection: 'connecting',
  games: [],
  status: null,
  installs: {},
  running: {},
  auth: {},
  error: null,
  mods: { status: null, games: [], account: null, job: null, log: [], authUrl: '', error: '' },

  setConnection: (connection) => set({ connection }),
  setGames: (games) => set({ games }),
  setStatus: (status) => set({ status }),
  setError: (error) => set({ error }),

  setAuth: (auth) => set({ auth }),

  setMods: (patch) => set((state) => ({ mods: { ...state.mods, ...patch } })),

  pushModsLog: (line) =>
    set((state) => ({ mods: { ...state.mods, log: [...state.mods.log, line].slice(-8) } })),

  // A finished job leaves the list; a failed one stays for the user to dismiss.
  applyProgress: (progress) =>
    set((state) => {
      const key = gameKey(progress.store, progress.id)
      const installs = { ...state.installs }
      if (progress.state === 'done') delete installs[key]
      else installs[key] = progress
      return { installs }
    }),

  clearProgress: (key) =>
    set((state) => {
      const installs = { ...state.installs }
      delete installs[key]
      return { installs }
    }),

  applyGameState: (event) =>
    set((state) => {
      const key = gameKey(event.store, event.id)
      const running = { ...state.running }
      if (event.state === 'stopped' || event.state === 'stopping') {
        delete running[key]
      } else {
        running[key] = event
      }
      return { running }
    }),

  // Mark what moved; games that are new arrive on the next library.list.
  applyLibraryChanged: (event) =>
    set((state) => {
      if (!event.updated?.length && !event.removed?.length) return {}
      const store = event.store ?? ''
      const updated = new Set(event.updated ?? [])
      const removed = new Set(event.removed ?? [])
      const games = state.games
        .filter((game) => !(removed.has(game.id) && (!store || game.store === store)))
        .map((game) =>
          updated.has(game.id) && (!store || game.store === store)
            ? { ...game, update_available: false }
            : game,
        )
      return { games }
    }),
}))
