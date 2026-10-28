import { rpc } from './link'
import type {
  ModsAccount,
  ModsBrowse,
  ModsFiles,
  ModsGame,
  ModsInstalled,
  ModMissing,
  ModFinding,
  ModsStatus,
  ModsUpdates,
} from './types'

export interface ModsTarget {
  game: string
  profile: string
}

export const modsStatus = () => rpc.call<ModsStatus>('mods.status', {})
export const modsGames = () => rpc.call<{ games?: ModsGame[] }>('mods.games', {})
export const modsAccount = () => rpc.call<ModsAccount>('mods.account', {})
export const modsSetup = () => rpc.call<{ job: string }>('mods.setup', {})
export const modsCancel = () => rpc.call<{ canceled: boolean }>('mods.cancel', {})
export const modsLogin = () => rpc.call<{ job: string }>('mods.login', {})
export const modsLogout = () => rpc.call<{ stored: boolean }>('mods.logout', {})
export const modsNexus = (key: string) => rpc.call<{ stored: boolean }>('mods.nexus', { action: 'set', key })
export const modsDeploy = (target: ModsTarget) => rpc.call<{ job: string }>('mods.deploy', target)
export const modsRestore = (game: string) => rpc.call<{ job: string }>('mods.restore', { game })

export const modsConfigure = (game: string, path: string) =>
  rpc.call<{ configured: boolean; problems?: string[] }>('mods.configure', { game, path })

export const modsInstalled = (target: ModsTarget) => rpc.call<ModsInstalled>('mods.installed', target)
export const modsUpdates = (target: ModsTarget) => rpc.call<ModsUpdates>('mods.updates', target)
export const modsUpdate = (target: ModsTarget, name: string) =>
  rpc.call<{ job: string }>('mods.update', { ...target, name })

export const modsBrowse = (game: string, query: string, sort: string, page: number) =>
  rpc.call<ModsBrowse>('mods.browse', { game, query, sort, page })

export const modsFiles = (game: string, modId: number) => rpc.call<ModsFiles>('mods.files', { game, mod_id: modId })

export const modsInstallFile = (target: ModsTarget, modId: number, fileId: number) =>
  rpc.call<{ job: string }>('mods.install', { ...target, mod_id: modId, file_id: fileId })

export const modsInstallArchive = (target: ModsTarget, archive: string, name?: string) =>
  rpc.call<{ job: string }>('mods.install', name ? { ...target, archive, name } : { ...target, archive })

export const modsToggle = (target: ModsTarget, name: string) =>
  rpc.call<{ name: string; enabled: boolean }>('mods.toggle', { ...target, name })

export const modsRemove = (target: ModsTarget, name: string) =>
  rpc.call<{ job: string }>('mods.remove', { ...target, name })

export const modsHealth = (target: ModsTarget) => rpc.call<{ findings?: ModFinding[] }>('mods.health', target)

export const modsRepair = (target: ModsTarget) =>
  rpc.call<{ job: string; detail?: string }>('mods.repair', target)

export const modsMissing = (target: ModsTarget) => rpc.call<{ missing?: ModMissing[] }>('mods.missing', target)

export const modsDownloads = (target: ModsTarget, modId: number, fileId: number) =>
  rpc.call<{ found: string }>('mods.downloads', { ...target, mod_id: modId, file_id: fileId })

export const nexusFilePage = (domain: string, modId: number, fileId: number) =>
  `https://www.nexusmods.com/${domain}/mods/${modId}?tab=files&file_id=${fileId}`
