export type Store = 'epic' | 'gog' | 'amazon' | 'steam' | 'local'

export interface Artwork {
  cover?: string
  square?: string
  logo?: string
  icon?: string
}

export interface Game {
  store: string
  id: string
  title: string
  developer?: string
  installed: boolean
  install_path?: string
  platform?: string
  version?: string
  executable?: string
  install_size?: string
  is_dlc?: boolean
  runner?: string
  source?: string
  artwork?: Artwork
  artwork_local?: Artwork
  cloud_saves?: boolean
  can_run_offline?: boolean
  update_available?: boolean
  favorite?: boolean
  last_played?: string
  play_count?: number
  metadata?: Record<string, string>
}

export interface LibraryListResult {
  games?: Game[]
}

export interface DaemonStatus {
  version: string
  commit: string
  uptime_seconds: number
  socket: string
  stores: StoreStatus[]
  engines: Record<string, EngineStatus>
}

export interface StoreStatus {
  store: string
  available: boolean
  logged_in?: boolean
  user?: string
}

export interface EngineStatus {
  available: boolean
  path?: string
  version?: string
  reason?: string
}

export interface InstallProgress {
  store: string
  id: string
  state: string
  percent?: number
  bytes?: number
  eta?: string
  down_speed?: string
  disk_speed?: string
}

export interface GameStateEvent {
  store: string
  id: string
  state: string
  pid?: number
  reason?: string
}

export interface LibraryChangedEvent {
  store?: string
  added?: string[]
  removed?: string[]
  updated?: string[]
}

export interface SecretStatus {
  name: string
  set: boolean
  encrypted: boolean
  backend: string
}

export interface SecretStatusResult {
  secrets: SecretStatus[]
  encryption: boolean
}

export interface GameKey {
  store: string
  id: string
}

export type ConnectionState = 'connecting' | 'live' | 'offline' | 'unauthorized'

export interface EndpointConfig {
  url: string
  token: string
  pid?: number
  palette?: PaletteRoles | null
}

/** The Material roles Ryoku's Tokens.qml resolves, as colours. */
export interface PaletteRoles {
  surface?: string
  surfaceContainerLow?: string
  onSurface?: string
  onSurfaceVariant?: string
  primary?: string
  inverseSurface?: string
  inverseOnSurface?: string
  [role: string]: string | undefined
}
