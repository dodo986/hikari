import { beforeEach, describe, expect, it, vi } from 'vitest'
import { hasHost, startDaemon } from '../src/host/bridge'

describe('the host bridge', () => {
  beforeEach(() => {
    vi.resetModules()
    delete window.hikari
    delete window.go
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
})
