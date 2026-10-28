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
  percent?: number | null
  bytes?: string
  eta?: string
  down_speed?: number | null
  disk_speed?: number | null
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

export interface InstallInfo {
  title?: string
  version?: string
  download_size: number
  disk_size: number
  dlcs?: { id: string; title: string; download_size?: number }[]
}

export interface LaunchOption {
  name: string
  type: string
  params?: string[]
  exe?: string
  dlc_app?: string
}

export interface WineVersion {
  name: string
  bin: string
  type: string
  source: string
  lib?: string
  lib32?: string
  wineserver?: string
  kind?: string
  path?: string
}

export interface EnvVar {
  key: string
  value: string
}

export interface Wrapper {
  kind: string
  enabled: boolean
}

export interface GameSettings {
  wine_version?: WineVersion
  wine_prefix: string
  target_exe: string
  launcher_args: string
  environment?: EnvVar[]
  wrappers?: Wrapper[]
  use_gamemode: boolean
  show_mangohud: boolean
  enable_esync: boolean
  enable_fsync: boolean
  enable_fsr: boolean
  max_sharpness: number
  auto_install_dxvk: boolean
  auto_install_vkd3d: boolean
  auto_install_dxvk_nvapi: boolean
  eac_runtime: boolean
  battleye_runtime: boolean
  enable_eos_overlay: boolean
  nvidia_prime: boolean
  show_fps: boolean
  enable_wine_wayland: boolean
  enable_hdr: boolean
  enable_wow64: boolean
  prefer_system_libs: boolean
  offline_mode: boolean
  disable_umu: boolean
  language: string
  saves_path?: string
  before_script: string
  after_script: string
}

export interface RunLog {
  store: string
  id: string
  running: boolean
  state?: string
  exit_code: number
  canceled?: boolean
  error?: string
  started_at?: string
  ended_at?: string
  lines: string[]
  mod_stop?: string
}

export interface AuthStatus {
  logged_in: boolean
  user?: string
  user_id?: string
}

export interface AuthStatusResult {
  stores: Record<string, AuthStatus>
}

export interface LoginStart {
  state: string
  url?: string
  hint?: string
  user?: { name?: string; id?: string }
}

export interface EngineStatus {
  available: boolean
  path?: string
  version?: string
}

export interface EngineVersions {
  engines: Record<string, EngineStatus>
}

export interface WineRelease {
  tag: string
  name: string
  variant: string
  kind: string
  date: string
  url: string
  size: number
}

export interface WineVersionsResult {
  versions: WineVersion[]
}

export interface OptiStatus {
  game_id: string
  installed: boolean
  version?: string
  injection_dll?: string
  int8?: boolean
  game_dir?: string
  launch_options?: string
}

export interface OptiGpu {
  vendor?: string
  name?: string
  supports_framegen?: boolean
  [key: string]: unknown
}

export interface OptiRelease {
  tag: string
  name?: string
  published?: string
  [key: string]: unknown
}

export interface OptiDetect {
  gpus?: OptiGpu[]
}
