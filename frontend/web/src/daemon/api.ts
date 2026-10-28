import { rpc } from './link'
import type { Game, GameStateEvent, LibraryListResult, SecretStatusResult } from './types'

export interface GameKeyParams extends Record<string, unknown> {
  store: string
  id: string
}

const key = (game: Pick<Game, 'store' | 'id'>): GameKeyParams => ({ store: game.store, id: game.id })

export const launchGame = (game: Pick<Game, 'store' | 'id'>) =>
  rpc.call<{ state: string }>('game.launch', key(game))

export const stopGame = (game: Pick<Game, 'store' | 'id'>) => rpc.call<unknown>('game.stop', key(game))

export const installGame = (game: Pick<Game, 'store' | 'id'>, options: Record<string, unknown> = {}) =>
  rpc.call<{ job: string; state: string }>('game.install', { ...key(game), ...options })

export const cancelInstall = (game: Pick<Game, 'store' | 'id'>) =>
  rpc.call<unknown>('game.install.cancel', key(game))

export const uninstallGame = (game: Pick<Game, 'store' | 'id'>, keepFiles = false) =>
  rpc.call<unknown>('game.uninstall', { ...key(game), keep_files: keepFiles })

export const repairGame = (game: Pick<Game, 'store' | 'id'>) => rpc.call<unknown>('game.repair', key(game))

export const favoriteGame = (game: Pick<Game, 'store' | 'id'>, favorite: boolean) =>
  rpc.call<unknown>('game.favorite', { ...key(game), favorite })

export const getGame = (game: Pick<Game, 'store' | 'id'>) =>
  rpc.call<{ game: Game }>('library.get', key(game))

export const getRunning = () => rpc.call<{ games: GameStateEvent[] }>('game.running', {})

export const refreshAll = () => rpc.call<{ ok: boolean }>('library.refresh', {})

export const searchArtwork = (title: string) =>
  rpc.call<{ candidates: { url: string; thumb?: string }[] }>('artwork.search', { title })

export interface SecretStatusReply {
  secrets: SecretStatusResult['secrets']
  encryption: boolean
}

export const secretStatus = (name?: string) =>
  rpc.call<SecretStatusReply>('settings.secretStatus', name ? { name } : {})

export const setSecret = (name: string, value: string) =>
  rpc.call<{ secret: SecretStatusResult['secrets'][number]; encryption: boolean }>(
    'settings.setSecret',
    { name, value },
  )

export type { LibraryListResult }
