import { beforeEach, describe, expect, it, vi } from 'vitest'
import { hasHost, onPalette, startDaemon } from '../src/host/bridge'

describe('the host bridge', () => {
  beforeEach(() => {
    vi.resetModules()
    delete window.hikari
    delete window.go
    delete window.runtime
  })

  it('reports no host in a plain browser', () => {
    expect(hasHost()).toBe(false)
  })

  it('uses the Wails binding when the window is native', async () => {
    const start = vi.fn(async () => {})
    window.go = { main: { Host: {
      PickFile: async () => '',
      PickFolder: async () => '',
      OpenUrl: async () => {},
      StartDaemon: start,
    } } }
    expect(hasHost()).toBe(true)
    await startDaemon()
    expect(start).toHaveBeenCalledOnce()
  })

  it('refuses to start a daemon it cannot reach', async () => {
    await expect(startDaemon()).rejects.toThrow('desktop window')
  })

  it('hands wallpaper palettes from the runtime to the theme', () => {
    const listeners: Record<string, (payload: unknown) => void> = {}
    window.runtime = {
      EventsOn: (name, callback) => {
        listeners[name] = callback
        return () => {}
      },
    }
    const seen: unknown[] = []
    onPalette((roles) => seen.push(roles))
    listeners.palette?.({ primary: '#ff0000' })
    expect(seen).toEqual([{ primary: '#ff0000' }])
  })
})
