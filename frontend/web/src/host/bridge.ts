// What the window adds over a browser: native dialogs and starting the daemon.
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

declare global {
  interface Window {
    go?: { main?: { Host?: WailsHostBinding } }
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
