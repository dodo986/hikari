/**
 * What the Wails window adds on top of a browser: native dialogs and the
 * ability to start the daemon. Everything else the page does itself over the
 * websocket, so the same bundle runs in a plain browser.
 */
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

export const hasHost = (): boolean => typeof window.hikari !== 'undefined'

export async function pickFile(label: string): Promise<string | null> {
  if (window.hikari) return window.hikari.pickFile(label)
  const typed = window.prompt(label)
  return typed && typed.trim() !== '' ? typed.trim() : null
}

export async function pickFolder(label: string): Promise<string | null> {
  if (window.hikari) return window.hikari.pickFolder(label)
  const typed = window.prompt(label)
  return typed && typed.trim() !== '' ? typed.trim() : null
}

export async function openUrl(url: string): Promise<void> {
  if (window.hikari) {
    await window.hikari.openUrl(url)
    return
  }
  window.open(url, '_blank', 'noopener')
}

export async function startDaemon(): Promise<void> {
  if (!window.hikari) throw new Error('starting hikarid needs the desktop window')
  await window.hikari.startDaemon()
}
