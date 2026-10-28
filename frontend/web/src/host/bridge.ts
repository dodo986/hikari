import type { PaletteRoles } from '../daemon/types'

// What the window adds over a browser: native dialogs, the palette and starting
// the daemon.
export interface HikariHost {
  pickFile(label: string): Promise<string | null>
  pickFolder(label: string): Promise<string | null>
  openUrl(url: string): Promise<void>
  startDaemon(): Promise<void>
}

declare global {
  interface Window {
    hikari?: HikariHost
  }
}

// Wails exposes Go methods as window.go.main.Host.<Method>, wrapped here.
interface WailsHostBinding {
  PickFile(label: string): Promise<string>
  PickFolder(label: string): Promise<string>
  OpenUrl(url: string): Promise<void>
  StartDaemon(): Promise<void>
}

// The Wails runtime carries the event bus; EventsOn returns its unsubscribe.
interface WailsRuntime {
  EventsOn(name: string, callback: (payload: unknown) => void): () => void
}

declare global {
  interface Window {
    go?: { main?: { Host?: WailsHostBinding } }
    runtime?: WailsRuntime
  }
}

function bindings(): WailsHostBinding | undefined {
  return window.go?.main?.Host
}

function host(): HikariHost | undefined {
  if (window.hikari) return window.hikari
  const bound = bindings()
  if (!bound) return undefined
  window.hikari = {
    pickFile: (label) => bound.PickFile(label),
    pickFolder: (label) => bound.PickFolder(label),
    openUrl: (url) => bound.OpenUrl(url),
    startDaemon: () => bound.StartDaemon(),
  }
  return window.hikari
}

export const hasHost = (): boolean => typeof host() !== 'undefined'

// A plain browser has no native dialog, so the path is typed instead.
function manualPath(label: string): string | null {
  const typed = window.prompt(label)
  return typed && typed.trim() !== '' ? typed.trim() : null
}

export async function pickFile(label: string): Promise<string | null> {
  const bound = host()
  return bound ? bound.pickFile(label) : manualPath(label)
}

export async function pickFolder(label: string): Promise<string | null> {
  const bound = host()
  return bound ? bound.pickFolder(label) : manualPath(label)
}

export async function openUrl(url: string): Promise<void> {
  const bound = host()
  if (bound) {
    await bound.openUrl(url)
    return
  }
  window.open(url, '_blank', 'noopener')
}

export async function startDaemon(): Promise<void> {
  const bound = host()
  if (!bound) throw new Error('starting hikarid needs the desktop window')
  await bound.startDaemon()
}

// Ryoku regenerates the palette when the wallpaper changes; the host watches
// that file and pushes the new roles here. The runtime script can land after
// the bundle, so a missing bus is retried before giving up.
export function onPalette(handler: (roles: PaletteRoles) => void): () => void {
  let stop: (() => void) | undefined
  let timer: number | undefined
  const attach = (tries: number): void => {
    const bus = window.runtime
    if (bus) {
      stop = bus.EventsOn('palette', (payload) => {
        if (payload) handler(payload as PaletteRoles)
      })
      return
    }
    if (tries > 0) timer = window.setTimeout(() => attach(tries - 1), 200)
  }
  attach(20)
  return () => {
    if (timer !== undefined) window.clearTimeout(timer)
    stop?.()
  }
}
