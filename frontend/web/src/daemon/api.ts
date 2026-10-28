import { rpc } from './link'
import type {
  AuthStatusResult,
  Game,
  GameSettings,
  InstallInfo,
  LaunchOption,
  LoginStart,
  OptiDetect,
  OptiRelease,
  OptiStatus,
  RunLog,
  SecretStatusResult,
  WineRelease,
  WineVersion,
} from './types'

export interface Key {
  store: string
  id: string
}

export const loadGame = (key: Key) => rpc.call<Game>('library.get', { ...key })

export const installInfo = (key: Key & { platform?: string }) =>
  rpc.call<InstallInfo>('game.install_info', { ...key })

export const launchOptions = (key: Key) =>
  rpc.call<{ options: LaunchOption[] }>('game.launch_options', { ...key })

export const updateGame = (key: Key, options: Record<string, unknown> = {}) =>
  rpc.call<{ job: string; state: string }>('game.update', { ...key, ...options })

export const runLog = (key: Key) => rpc.call<RunLog>('game.log', { ...key })

export const syncSaves = (key: Key, direction?: 'download' | 'upload') =>
  rpc.call<{ ok: boolean; message?: string }>('game.saves.sync', { ...key, direction })

export const gameSettings = (key: Key) => rpc.call<GameSettings>('game.settings.get', { ...key })

export const repairGame = (key: Key) => rpc.call<unknown>('game.repair', { ...key })

export const saveGameSettings = (key: Key, settings: GameSettings) =>
  rpc.call<unknown>('game.settings.set', { ...key, settings })

export const authStatus = () => rpc.call<AuthStatusResult>('auth.status', {})

export const beginLogin = (store: string) => rpc.call<LoginStart>('auth.login', { store })

export const completeLogin = (store: string, code: string) =>
  rpc.call<LoginStart>('auth.login', { store, code })

export const autoLogin = (store: string) => rpc.call<LoginStart>('auth.login.auto', { store })

export const logout = (store: string) => rpc.call<unknown>('auth.logout', { store })


export const wineVersions = () => rpc.call<{ versions: WineVersion[] }>('wine.versions', {})

export const wineReleases = () => rpc.call<{ releases: WineRelease[] }>('wine.releases', {})

export const installWine = (tag: string) => rpc.call<{ job: string }>('wine.download', { tag })

export const removeWine = (name: string) => rpc.call<unknown>('wine.remove', { name })

export const optiStatus = (key: Key) => rpc.call<OptiStatus>('opti.status', { ...key })

export const optiDetect = () => rpc.call<OptiDetect>('opti.detect', {})

export const optiReleases = () => rpc.call<{ releases: OptiRelease[] }>('opti.releases', {})


export const optiInstall = (key: Key, options: Record<string, unknown>) =>
  rpc.call<{ job: string }>('opti.install', { ...key, ...options })

export const optiRemove = (key: Key) => rpc.call<unknown>('opti.uninstall', { ...key })

export const tailLog = (lines = 200) => rpc.call<{ path: string; lines: string[] }>('logs.tail', { lines })

export const report = () => rpc.call<{ text: string }>('logs.report', {})

export const checkUpdate = () => rpc.call<{ available: boolean; version?: string; notes?: string }>('app.update.check', {})

export const applyUpdate = () => rpc.call<{ started: boolean }>('app.update.apply', {})

export interface LaunchPlan {
  env?: Record<string, string>
  argv?: string[]
  error?: string
}

export const dryRun = (key: Key) => rpc.call<{ plan?: LaunchPlan }>('game.launch', { ...key, dry_run: true })

export const launchWith = (key: Key, option?: LaunchOption) =>
  rpc.call<{ state: string }>('game.launch', option ? { ...key, option } : { ...key })

export const installTo = (key: Key, path: string, platform: string, dlcs: string[]) =>
  rpc.call<{ job: string; state: string }>('game.install', { ...key, path, platform, dlcs })

export const removeLocal = (id: string) => rpc.call<unknown>('game.remove_local', { id })

export const setFavorite = (key: Key, favorite: boolean) =>
  rpc.call<{ favorite: boolean }>('game.favorite', { ...key, favorite })

export const stopGame = (key: Key) => rpc.call<unknown>('game.stop', { ...key })

export const cancelInstall = (key: Key) => rpc.call<unknown>('game.install.cancel', { ...key })


export const refreshAll = () => rpc.call<{ ok: boolean }>('library.refresh', {})

export const searchArtwork = (title: string) =>
  rpc.call<{ candidates: { url: string; thumb?: string }[] }>('artwork.search', { title })

export const secretStatus = (name?: string) =>
  rpc.call<SecretStatusResult>('settings.secretStatus', name ? { name } : {})

export const setSecret = (name: string, value: string) =>
  rpc.call<{ secret: SecretStatusResult['secrets'][number]; encryption: boolean }>(
    'settings.setSecret',
    { name, value },
  )

export const deleteSecret = (name: string) =>
  rpc.call<{ deleted: boolean }>('settings.deleteSecret', { name })
